from fastapi import FastAPI
from pydantic import BaseModel
from text_to_sql import query
import sqlite3

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

if __name__ == '__main__':
    import uvicorn
    uvicorn.run(app, host='0.0.0.0', port=8002)
