from pydantic import BaseModel
from datetime import datetime
from typing import Optional


class FileResponse(BaseModel):
    id: int
    filename: str
    program: str
    version: str
    os: str
    arch: str
    file_size: int
    uploaded_at: datetime

    class Config:
        from_attributes = True


class FileListResponse(BaseModel):
    files: list[FileResponse]


class AgentRegisterRequest(BaseModel):
    agent_id: str
    hostname: str
    version: str
    os: str
    arch: str
    ip: Optional[str] = None
    pushgateway_url: str
    push_interval_seconds: int
    node_exporter_port: int
    node_exporter_metrics_url: str
    started_at: datetime


class AgentHeartbeatRequest(BaseModel):
    agent_id: str
    status: str
    last_error: Optional[str] = None
    last_push_at: Optional[datetime] = None
    last_push_success_at: Optional[datetime] = None
    last_push_error_at: Optional[datetime] = None
    push_fail_count: int = 0
    node_exporter_up: bool = True


class AgentEventResponse(BaseModel):
    event_type: str
    message: str
    created_at: datetime

    class Config:
        from_attributes = True


class AgentResponse(BaseModel):
    agent_id: str
    hostname: str
    version: str
    os: str
    arch: str
    ip: Optional[str] = None
    status: str
    online: bool
    last_error: Optional[str] = None
    pushgateway_url: str
    push_interval_seconds: int
    node_exporter_port: int
    node_exporter_metrics_url: str
    node_exporter_up: bool
    push_fail_count: int
    started_at: Optional[datetime] = None
    last_seen_at: Optional[datetime] = None
    last_push_at: Optional[datetime] = None
    last_push_success_at: Optional[datetime] = None
    last_push_error_at: Optional[datetime] = None
    registered_at: datetime
    updated_at: datetime


class AgentListResponse(BaseModel):
    agents: list[AgentResponse]


class AgentDetailResponse(BaseModel):
    agent: AgentResponse
    events: list[AgentEventResponse]


class AgentRegisterResponse(BaseModel):
    heartbeat_interval_seconds: int
    offline_timeout_seconds: int
