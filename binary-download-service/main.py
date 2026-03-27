import os
import re
import shutil
from datetime import datetime, timedelta, timezone
from fastapi import FastAPI, Depends, HTTPException, UploadFile, File
from fastapi.responses import FileResponse as FastAPIFileResponse
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy.orm import Session
from typing import Optional

from database import (
    AgentEventRecord,
    AgentRecord,
    FileRecord,
    get_db,
    localnow,
)
from schemas import (
    AgentDetailResponse,
    AgentEventResponse,
    AgentHeartbeatRequest,
    AgentListResponse,
    AgentRegisterRequest,
    AgentRegisterResponse,
    AgentResponse,
    FileListResponse,
    FileResponse as FileRecordResponse,
)

app = FastAPI(title="Binary Download Service")
INSTALL_SCRIPT_NAME = "install.sh"
UNINSTALL_SCRIPT_NAME = "uninstall.sh"
INSTALL_SCRIPT_PROGRAM = "install-script"
HEARTBEAT_INTERVAL_SECONDS = 30
OFFLINE_TIMEOUT_SECONDS = 90
RECENT_EVENTS_LIMIT = 20

# 添加 CORS 支持
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# 解析文件名 - 兼容 `{version}-{os}-{arch}` 和 `{version}.{os}-{arch}` 两种格式
FILENAME_PATTERN = re.compile(
    r"^(?P<program>node_exporter|node-push-exporter)-(?P<version>.+?)[.-](?P<os>linux|darwin)-(?P<arch>amd64|arm64)(?P<ext>\.tar\.gz)?$"
)

UPLOAD_DIR = os.path.join(os.path.dirname(__file__), "uploads")
os.makedirs(UPLOAD_DIR, exist_ok=True)
STATIC_DIR = os.path.join(os.path.dirname(__file__), "static")


@app.get("/api/files", response_model=FileListResponse)
def list_files(program: Optional[str] = None, db: Session = Depends(get_db)):
    query = db.query(FileRecord)
    if program:
        query = query.filter(FileRecord.program == program)
    files = query.order_by(FileRecord.uploaded_at.desc()).all()
    return FileListResponse(files=[FileRecordResponse.model_validate(f) for f in files])


@app.delete("/api/files/{file_id}")
def delete_file(file_id: int, db: Session = Depends(get_db)):
    file_record = db.query(FileRecord).filter(FileRecord.id == file_id).first()
    if not file_record:
        raise HTTPException(status_code=404, detail="文件不存在")

    # 删除物理文件
    if os.path.exists(file_record.file_path):
        os.remove(file_record.file_path)

    # 删除数据库记录
    db.delete(file_record)
    db.commit()

    return {"message": "文件删除成功"}


def parse_filename(filename: str) -> dict:
    """解析文件名提取 program, version, os, arch"""
    if filename == INSTALL_SCRIPT_NAME or filename == UNINSTALL_SCRIPT_NAME:
        return {
            "program": INSTALL_SCRIPT_PROGRAM,
            "version": filename,
            "os": "script",
            "arch": "shell",
        }

    match = FILENAME_PATTERN.match(filename)
    if match:
        return {
            "program": match.group("program"),
            "version": match.group("version"),
            "os": match.group("os"),
            "arch": match.group("arch"),
        }

    # 如果不匹配格式，尝试从文件名推断 program
    if filename.startswith("node_exporter"):
        program = "node_exporter"
    elif filename.startswith("node-push-exporter"):
        program = "node-push-exporter"
    else:
        program = "unknown"

    return {
        "program": program,
        "version": filename,
        "os": "unknown",
        "arch": "unknown",
    }


def utcnow() -> datetime:
    return localnow()


def normalize_datetime(value: Optional[datetime]) -> Optional[datetime]:
    if value is None:
        return None
    if value.tzinfo is not None:
        return value.astimezone(timezone(timedelta(hours=8))).replace(tzinfo=None)
    return value


def add_agent_event(db: Session, agent_id: str, event_type: str, message: str):
    db.add(
        AgentEventRecord(
            agent_id=agent_id,
            event_type=event_type,
            message=message,
        )
    )


def serialize_agent(agent: AgentRecord) -> AgentResponse:
    now = utcnow()
    last_seen = agent.last_seen_at
    online = False
    effective_status = "offline"
    if last_seen is not None and now-last_seen <= timedelta(seconds=OFFLINE_TIMEOUT_SECONDS):
        online = True
        degraded = (
            agent.status == "degraded"
            or not agent.node_exporter_up
            or agent.push_fail_count > 0
            or bool(agent.last_error)
        )
        effective_status = "degraded" if degraded else "online"

    return AgentResponse(
        agent_id=agent.agent_id,
        hostname=agent.hostname,
        version=agent.version,
        os=agent.os,
        arch=agent.arch,
        ip=agent.ip,
        status=effective_status,
        online=online,
        last_error=agent.last_error,
        pushgateway_url=agent.pushgateway_url,
        push_interval_seconds=agent.push_interval_seconds,
        node_exporter_port=agent.node_exporter_port,
        node_exporter_metrics_url=agent.node_exporter_metrics_url,
        node_exporter_up=agent.node_exporter_up,
        push_fail_count=agent.push_fail_count,
        started_at=agent.started_at,
        last_seen_at=agent.last_seen_at,
        last_push_at=agent.last_push_at,
        last_push_success_at=agent.last_push_success_at,
        last_push_error_at=agent.last_push_error_at,
        registered_at=agent.registered_at,
        updated_at=agent.updated_at,
    )


@app.post("/api/upload")
async def upload_file(file: UploadFile = File(...), db: Session = Depends(get_db)):
    # 解析文件名
    parsed = parse_filename(file.filename)

    # 保存文件
    file_path = os.path.join(UPLOAD_DIR, file.filename)

    # 同名文件视为替换：先删旧记录并立刻 flush，释放 filename 唯一索引
    old_record = db.query(FileRecord).filter(FileRecord.filename == file.filename).first()
    if old_record:
        if os.path.exists(old_record.file_path):
            os.remove(old_record.file_path)
        db.delete(old_record)
        db.flush()

    with open(file_path, "wb") as buffer:
        shutil.copyfileobj(file.file, buffer)

    file_size = os.path.getsize(file_path)

    # 保存到数据库
    file_record = FileRecord(
        filename=file.filename,
        program=parsed["program"],
        version=parsed["version"],
        os=parsed["os"],
        arch=parsed["arch"],
        file_path=file_path,
        file_size=file_size,
    )
    db.add(file_record)
    db.commit()
    db.refresh(file_record)

    return {
        "id": file_record.id,
        "filename": file_record.filename,
        "message": "文件上传成功"
    }


@app.get("/download/{filename}")
def download_file(filename: str, db: Session = Depends(get_db)):
    file_record = db.query(FileRecord).filter(FileRecord.filename == filename).first()
    if not file_record or not os.path.exists(file_record.file_path):
        raise HTTPException(status_code=404, detail="文件不存在")

    return FastAPIFileResponse(
        path=file_record.file_path,
        filename=filename,
        media_type="application/octet-stream"
    )


@app.post(
    "/api/agents/register",
    response_model=AgentRegisterResponse,
)
def register_agent(payload: AgentRegisterRequest, db: Session = Depends(get_db)):
    record = db.query(AgentRecord).filter(AgentRecord.agent_id == payload.agent_id).first()
    is_new = record is None
    if record is None:
        record = AgentRecord(agent_id=payload.agent_id)
        db.add(record)

    record.hostname = payload.hostname
    record.version = payload.version
    record.os = payload.os
    record.arch = payload.arch
    record.ip = payload.ip
    record.status = "online"
    record.last_error = None
    record.pushgateway_url = payload.pushgateway_url
    record.push_interval_seconds = payload.push_interval_seconds
    record.node_exporter_port = payload.node_exporter_port
    record.node_exporter_metrics_url = payload.node_exporter_metrics_url
    record.node_exporter_up = True
    record.push_fail_count = 0
    record.started_at = normalize_datetime(payload.started_at)
    record.last_seen_at = utcnow()

    add_agent_event(
        db,
        payload.agent_id,
        "registered" if is_new else "reregistered",
        f"{payload.hostname} 已完成注册",
    )
    db.commit()

    return AgentRegisterResponse(
        heartbeat_interval_seconds=HEARTBEAT_INTERVAL_SECONDS,
        offline_timeout_seconds=OFFLINE_TIMEOUT_SECONDS,
    )


@app.post("/api/agents/heartbeat")
def heartbeat_agent(payload: AgentHeartbeatRequest, db: Session = Depends(get_db)):
    record = db.query(AgentRecord).filter(AgentRecord.agent_id == payload.agent_id).first()
    if record is None:
        raise HTTPException(status_code=404, detail="节点未注册")

    previous_status = record.status
    previous_error = record.last_error

    record.status = payload.status
    record.last_error = payload.last_error
    record.last_push_at = normalize_datetime(payload.last_push_at)
    record.last_push_success_at = normalize_datetime(payload.last_push_success_at)
    record.last_push_error_at = normalize_datetime(payload.last_push_error_at)
    record.push_fail_count = payload.push_fail_count
    record.node_exporter_up = payload.node_exporter_up
    record.last_seen_at = utcnow()

    if previous_status != payload.status:
        add_agent_event(
            db,
            payload.agent_id,
            "status_changed",
            f"状态从 {previous_status} 变更为 {payload.status}",
        )
    elif payload.last_error and payload.last_error != previous_error:
        add_agent_event(
            db,
            payload.agent_id,
            "error",
            payload.last_error,
        )

    db.commit()
    return {"message": "心跳更新成功"}


@app.get("/api/agents", response_model=AgentListResponse)
def list_agents(db: Session = Depends(get_db)):
    rows = db.query(AgentRecord).order_by(AgentRecord.updated_at.desc()).all()
    return AgentListResponse(agents=[serialize_agent(row) for row in rows])


@app.get("/api/agents/{agent_id}", response_model=AgentDetailResponse)
def get_agent(agent_id: str, db: Session = Depends(get_db)):
    record = db.query(AgentRecord).filter(AgentRecord.agent_id == agent_id).first()
    if record is None:
        raise HTTPException(status_code=404, detail="节点不存在")

    events = (
        db.query(AgentEventRecord)
        .filter(AgentEventRecord.agent_id == agent_id)
        .order_by(AgentEventRecord.created_at.desc())
        .limit(RECENT_EVENTS_LIMIT)
        .all()
    )
    return AgentDetailResponse(
        agent=serialize_agent(record),
        events=[AgentEventResponse.model_validate(event) for event in events],
    )


@app.get("/agents")
def agents_page():
    return FastAPIFileResponse(os.path.join(STATIC_DIR, "agents.html"))


# 挂载静态文件
app.mount("/", StaticFiles(directory=STATIC_DIR, html=True), name="static")
