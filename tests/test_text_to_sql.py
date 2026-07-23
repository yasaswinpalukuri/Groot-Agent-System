import sys
import os
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))
from unittest import mock

def test_query_returns_dict():
    from text_to_sql import query
    mock_response = mock.Mock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "message": {"content": "SELECT * FROM jobs LIMIT 5;"}
    }
    mock_response.raise_for_status = mock.Mock()
    with mock.patch('requests.post', return_value=mock_response):
        result = query("Show me all jobs")
    assert isinstance(result, dict)
    assert "question" in result
    assert "sql" in result

def test_query_has_sql_key():
    from text_to_sql import query
    mock_response = mock.Mock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "message": {"content": "SELECT company FROM jobs WHERE score > 7;"}
    }
    mock_response.raise_for_status = mock.Mock()
    with mock.patch('requests.post', return_value=mock_response):
        result = query("High score jobs")
    assert "sql" in result
    assert "SELECT" in result.get("sql", "").upper()

def test_query_error_handling():
    from text_to_sql import query
    with mock.patch('requests.post', side_effect=Exception("Connection refused")):
        result = query("test question")
    assert "error" in result
    assert result["question"] == "test question"
