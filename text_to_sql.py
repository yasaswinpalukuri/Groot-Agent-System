import sqlite3
import requests
import re

OLLAMA_URL = "http://localhost:11434/api/chat"
DB_PATH    = "/home/groot/code/tony/jobs.db"

def query(question: str) -> dict:
    # Ask qwen2.5-coder to generate SQL
    try:
        response = requests.post(OLLAMA_URL, json={
            "model": "qwen2.5-coder:7b",
            "messages": [
                {
                    "role": "system",
                    "content": (
                        "You are a SQL expert. Generate ONLY a valid SQLite SELECT query "
                        "for a table called 'jobs' with columns: id, company, role, location, "
                        "salary, status, date_applied, score. "
                        "Return ONLY the SQL query with no explanation, no markdown, no backticks."
                    )
                },
                {"role": "user", "content": question}
            ],
            "stream": False,
        }, timeout=60)
        response.raise_for_status()
        sql = response.json()["message"]["content"].strip()
        # Clean up any markdown code blocks Tony might include
        sql = re.sub(r"```sql|```", "", sql).strip()
    except Exception as e:
        return {"question": question, "error": str(e), "sql": None}

    # Execute SQL against jobs.db
    try:
        conn    = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        cursor  = conn.cursor()
        cursor.execute(sql)
        rows    = [dict(row) for row in cursor.fetchall()]
        conn.close()
        return {
            "question":  question,
            "sql":       sql,
            "results":   rows,
            "row_count": len(rows),
        }
    except Exception as e:
        return {"question": question, "sql": sql, "error": str(e)}

if __name__ == "__main__":
    tests = [
        "Show me all jobs in Toronto",
        "Which jobs have a score above 7?",
        "What companies have applied jobs?",
    ]
    for q in tests:
        result = query(q)
        print(f"\nQ: {q}")
        if "error" in result:
            print(f"Error: {result['error']}")
        else:
            print(f"SQL: {result['sql']}")
            print(f"Rows: {result['row_count']}")
            for row in result['results'][:3]:
                print(f"  {row}")
