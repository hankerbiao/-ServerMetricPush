import os
from datetime import datetime
from zoneinfo import ZoneInfo

from sqlalchemy import Boolean, Text, create_engine
from sqlalchemy.orm import sessionmaker, declarative_base
from sqlalchemy import Column, Integer, String, DateTime

SHANGHAI_TZ = ZoneInfo("Asia/Shanghai")


def localnow() -> datetime:
    return datetime.now(SHANGHAI_TZ).replace(tzinfo=None)

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
DATABASE_URL = f"sqlite:///{os.path.join(BASE_DIR, 'files.db')}"

engine = create_engine(DATABASE_URL, connect_args={"check_same_thread": False})
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()


class FileRecord(Base):
    __tablename__ = "files"

    id = Column(Integer, primary_key=True, index=True)
    filename = Column(String, unique=True, nullable=False)
    program = Column(String, nullable=False)
    version = Column(String, nullable=False)
    os = Column(String, nullable=False)
    arch = Column(String, nullable=False)
    file_path = Column(String, nullable=False)
    file_size = Column(Integer, nullable=False)
    uploaded_at = Column(DateTime, default=localnow)


class AgentRecord(Base):
    __tablename__ = "agents"

    id = Column(Integer, primary_key=True, index=True)
    agent_id = Column(String, unique=True, nullable=False, index=True)
    hostname = Column(String, nullable=False)
    version = Column(String, nullable=False)
    os = Column(String, nullable=False)
    arch = Column(String, nullable=False)
    ip = Column(String, nullable=True)
    status = Column(String, nullable=False, default="online")
    last_error = Column(Text, nullable=True)
    pushgateway_url = Column(String, nullable=False)
    push_interval_seconds = Column(Integer, nullable=False)
    node_exporter_port = Column(Integer, nullable=False)
    node_exporter_metrics_url = Column(String, nullable=False)
    node_exporter_up = Column(Boolean, nullable=False, default=True)
    push_fail_count = Column(Integer, nullable=False, default=0)
    started_at = Column(DateTime, nullable=True)
    last_seen_at = Column(DateTime, nullable=True)
    last_push_at = Column(DateTime, nullable=True)
    last_push_success_at = Column(DateTime, nullable=True)
    last_push_error_at = Column(DateTime, nullable=True)
    registered_at = Column(DateTime, default=localnow)
    updated_at = Column(DateTime, default=localnow, onupdate=localnow)


class AgentEventRecord(Base):
    __tablename__ = "agent_events"

    id = Column(Integer, primary_key=True, index=True)
    agent_id = Column(String, nullable=False, index=True)
    event_type = Column(String, nullable=False)
    message = Column(Text, nullable=False)
    created_at = Column(DateTime, default=localnow)


Base.metadata.create_all(bind=engine)


def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
