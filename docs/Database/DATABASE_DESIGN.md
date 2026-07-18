# Database Design — Future Direction

PostgreSQL is the planned durable store for structured platform data. Start with a small core only when persistence is needed: repositories, scans, scan artifacts, evidence references, decisions, investigations, validation runs, and audit events.

Qdrant is a future retrieval index, not the source of truth. Do not add either database to the Repository Scanner until a demonstrated product need requires persistence.
