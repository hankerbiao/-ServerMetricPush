import os
import unittest
import uuid

from fastapi.testclient import TestClient

from database import AgentEventRecord, AgentRecord, SessionLocal
from main import app


class AgentsAPITests(unittest.TestCase):
    def setUp(self):
        self.agent_id = f"agent-{uuid.uuid4().hex}"
        self.client = TestClient(app)

    def tearDown(self):
        self.client.close()
        db = SessionLocal()
        try:
            db.query(AgentEventRecord).filter(AgentEventRecord.agent_id == self.agent_id).delete()
            db.query(AgentRecord).filter(AgentRecord.agent_id == self.agent_id).delete()
            db.commit()
        finally:
            db.close()

    def test_register_creates_agent_and_returns_intervals(self):
        response = self.client.post(
            "/api/agents/register",
            json=self.register_payload(),
        )

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json()["heartbeat_interval_seconds"], 30)
        self.assertEqual(response.json()["offline_timeout_seconds"], 90)

        db = SessionLocal()
        try:
            row = db.query(AgentRecord).filter(AgentRecord.agent_id == self.agent_id).one()
            self.assertEqual(row.hostname, "host-01")
            self.assertEqual(row.status, "online")
            self.assertEqual(row.version, "1.2.3")
        finally:
            db.close()

    def test_register_updates_existing_agent(self):
        first = self.client.post(
            "/api/agents/register",
            json=self.register_payload(),
        )
        self.assertEqual(first.status_code, 200)

        updated_payload = self.register_payload()
        updated_payload["hostname"] = "host-02"
        updated_payload["version"] = "1.2.4"
        second = self.client.post(
            "/api/agents/register",
            json=updated_payload,
        )

        self.assertEqual(second.status_code, 200)

        db = SessionLocal()
        try:
            rows = db.query(AgentRecord).filter(AgentRecord.agent_id == self.agent_id).all()
            self.assertEqual(len(rows), 1)
            self.assertEqual(rows[0].hostname, "host-02")
            self.assertEqual(rows[0].version, "1.2.4")
        finally:
            db.close()

    def test_heartbeat_updates_agent_status_and_list(self):
        registered = self.client.post(
            "/api/agents/register",
            json=self.register_payload(),
        )
        self.assertEqual(registered.status_code, 200)

        heartbeat = self.client.post(
            "/api/agents/heartbeat",
            json={
                "agent_id": self.agent_id,
                "status": "degraded",
                "last_error": "push failed",
                "last_push_at": "2026-03-27T12:00:00Z",
                "last_push_success_at": "2026-03-27T11:59:00Z",
                "last_push_error_at": "2026-03-27T12:00:00Z",
                "push_fail_count": 2,
                "node_exporter_up": False,
            },
        )

        listing = self.client.get("/api/agents")

        self.assertEqual(heartbeat.status_code, 200)
        self.assertEqual(listing.status_code, 200)
        self.assertEqual(len(listing.json()["agents"]), 1)
        agent = listing.json()["agents"][0]
        self.assertEqual(agent["status"], "degraded")
        self.assertEqual(agent["last_error"], "push failed")
        self.assertFalse(agent["node_exporter_up"])
        self.assertTrue(agent["online"])

    def register_payload(self):
        return {
            "agent_id": self.agent_id,
            "hostname": "host-01",
            "version": "1.2.3",
            "os": "linux",
            "arch": "amd64",
            "ip": "10.0.0.1",
            "pushgateway_url": "http://pushgateway:9091",
            "push_interval_seconds": 30,
            "node_exporter_port": 9100,
            "node_exporter_metrics_url": "http://127.0.0.1:9100/metrics",
            "started_at": "2026-03-27T11:58:00Z",
        }


if __name__ == "__main__":
    unittest.main()
