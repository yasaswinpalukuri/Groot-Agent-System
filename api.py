from fastapi import FastAPI, Request, HTTPException
from pydantic import BaseModel
from text_to_sql import query
import sqlite3
import requests
import time
from datetime import datetime
import os
from evaluator import evaluate

app = FastAPI()

DB_PATH = '/home/groot/code/tony/jobs.db'

class QueryRequest(BaseModel):
    question: str

@app.post("/query")
async def post_query(req: QueryRequest):
    result = query(req.question)
    return result

@app.get("/health")
async def get_health():
    return {"status": "ok", "model": "qwen2.5-coder:7b"}

@app.get("/jobs")
async def get_jobs():
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("SELECT * FROM jobs")
    rows = cursor.fetchall()
    columns = [description[0] for description in cursor.description]
    results = [dict(zip(columns, row)) for row in rows]
    conn.close()
    return results

@app.post("/evaluate")
async def post_evaluate(request: Request):
    data = await request.json()
    model = data.get("model", "qwen2.5-coder:7b")
    base_url = data.get("base_url", "http://localhost:8003")
    
    summary = evaluate(model, base_url)
    return summary

@app.get("/reports")
async def get_reports():
    reports_dir = "reports"
    if not os.path.exists(reports_dir):
        os.makedirs(reports_dir)
    
    report_files = [f for f in os.listdir(reports_dir) if f.endswith(".md")]
    return {"reports": report_files}

if __name__ == '__main__':
    import uvicorn
    uvicorn.run(app, host='0.0.0.0', port=8003)
