import sys
import requests
import json

def fetch_jobs(job_title, location):
    url = f"https://jobbank.gc.ca/api/jobs?keywords={job_title}&location={location}&distance=25&lang=en"
    headers = {
        "User-Agent": "GrootJobSearch/1.0"
    }
    
    response = requests.get(url, headers=headers)
    if response.status_code != 200:
        print(f"Failed to fetch jobs: {response.status_code}")
        return []
    
    data = response.json()
    jobs = []
    for job in data['jobPostings']:
        title = job['title']
        company = job['employer_name']
        location_city = job['location_city']
        job_url = job['job_url']
        date_posted = job['date_posted']
        
        jobs.append({
            'title': title,
            'company': company,
            'location': location_city,
            'url': job_url,
            'date': date_posted
        })
    
    return jobs

def save_jobs_to_json(jobs):
    with open('jobs_output.json', 'w') as f:
        json.dump(jobs, f, indent=4)

def main():
    if len(sys.argv) != 3:
        print("Usage: python job_scraper.py <job_title> <location>")
        sys.exit(1)
    
    job_title = sys.argv[1]
    location = sys.argv[2]
    
    jobs = fetch_jobs(job_title, location)
    save_jobs_to_json(jobs)
    
    print(f"Found {len(jobs)} jobs.")

if __name__ == '__main__':
    main()
