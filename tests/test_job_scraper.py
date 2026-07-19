import json
import os
import sys
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))

def test_scraper_imports():
    import job_scraper
    assert hasattr(job_scraper, 'fetch_jobs')
    assert hasattr(job_scraper, 'main')

def test_fetch_jobs_returns_list():
    from job_scraper import fetch_jobs
    # Mock test — no real API call
    import unittest.mock as mock
    mock_response = mock.Mock()
    mock_response.status_code = 200
    mock_response.json.return_value = {
        "results": [
            {
                "title": "Data Engineer",
                "company": {"display_name": "TestCorp"},
                "location": {"display_name": "Toronto, Ontario"},
                "redirect_url": "https://example.com",
                "created": "2026-07-19T00:00:00Z",
                "salary_min": 100000,
                "salary_max": 130000,
                "category": {"label": "IT Jobs"},
            }
        ]
    }
    with mock.patch('requests.get', return_value=mock_response):
        os.environ['ADZUNA_APP_ID'] = 'test'
        os.environ['ADZUNA_APP_KEY'] = 'test'
        jobs = fetch_jobs("Data Engineer", "Toronto")
    assert isinstance(jobs, list)
    assert len(jobs) == 1
    assert jobs[0]['company'] == 'TestCorp'
    assert jobs[0]['title'] == 'Data Engineer'

def test_job_has_required_fields():
    from job_scraper import fetch_jobs
    import unittest.mock as mock
    mock_response = mock.Mock()
    mock_response.status_code = 200
    mock_response.json.return_value = {"results": [
        {"title": "ML Eng", "company": {"display_name": "AI Corp"},
         "location": {"display_name": "Vancouver"},
         "redirect_url": "https://example.com",
         "created": "2026-07-19T00:00:00Z",
         "category": {"label": "IT Jobs"}}
    ]}
    with mock.patch('requests.get', return_value=mock_response):
        os.environ['ADZUNA_APP_ID'] = 'test'
        os.environ['ADZUNA_APP_KEY'] = 'test'
        jobs = fetch_jobs("ML Engineer", "Vancouver")
    required = ['title', 'company', 'location', 'url', 'date']
    for field in required:
        assert field in jobs[0], f"Missing field: {field}"
