import os
import re
import shutil
from fastapi import FastAPI, Depends, HTTPException, UploadFile, File
from fastapi.responses import FileResponse
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware
from sqlalchemy.orm import Session
from typing import Optional

from database import get_db, FileRecord, Base, engine
from schemas import FileResponse, FileListResponse

app = FastAPI(title="Binary Download Service")

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


@app.get("/api/files", response_model=FileListResponse)
def list_files(program: Optional[str] = None, db: Session = Depends(get_db)):
    query = db.query(FileRecord)
    if program:
        query = query.filter(FileRecord.program == program)
    files = query.order_by(FileRecord.uploaded_at.desc()).all()
    return FileListResponse(files=[FileResponse.model_validate(f) for f in files])


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

    return FileResponse(
        path=file_record.file_path,
        filename=filename,
        media_type="application/octet-stream"
    )


# 挂载静态文件
app.mount("/", StaticFiles(directory=os.path.join(os.path.dirname(__file__), "static"), html=True), name="static")
