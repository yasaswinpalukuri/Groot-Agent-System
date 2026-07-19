import sys
import os
import json
import requests

def fetch_jobs(job_title, location, results=20):
    app_id  = os.environ.get("ADZUNA_APP_ID")
    app_key = os.environ.get("ADZUNA_APP_KEY")

    if not app_id or not app_key:
        print("Error: ADZUNA_APP_ID and ADZUNA_APP_KEY not set")
        sys.exit(1)

    url = "https://api.adzuna.com/v1/api/jobs/ca/search/1"
    params = {
        "app_id":          app_id,
        "app_key":         app_key,
        "results_per_page": results,
        "what":            job_title,
        "where":           location,
        "content-type":    "application/json",
    }

    response = requests.get(url, params=params)
    if response.status_code != 200:
        print(f"Adzuna API error: {response.status_code} — {response.text[:200]}")
        return []

    data = response.json()
    jobs = []
    for job in data.get("results", []):
        jobs.append({
            "title":    job.get("title", ""),
            "company":  job.get("company", {}).get("display_name", ""),
            "location": job.get("location", {}).get("display_name", ""),
            "url":      job.get("redirect_url", ""),
            "date":     job.get("created", ""),
            "salary":   f"${job.get('salary_min',0):,.0f} - ${job.get('salary_max',0):,.0f}" if job.get('salary_min') else "Not specified",
            "category": job.get("category", {}).get("label", ""),
        })
    return jobs

def main():
    if len(sys.argv) < 3:
        print("Usage: python3 job_scraper.py <job_title> <location>")
        sys.exit(1)

    job_title = sys.argv[1]
    location  = sys.argv[2]

    print(f"Searching Adzuna Canada for '{job_title}' in '{location}'...")
    jobs = fetch_jobs(job_title, location)

    with open("jobs_output.json", "w") as f:
        json.dump(jobs, f, indent=2)

    print(f"Found {len(jobs)} jobs. Saved to jobs_output.json")
    for i, job in enumerate(jobs[:5], 1):
        print(f"\n{i}. {job['title']} @ {job['company']}")
        print(f"   {job['location']} | {job['salary']}")
        print(f"   {job['url'][:80]}")

if __name__ == "__main__":
    main()
