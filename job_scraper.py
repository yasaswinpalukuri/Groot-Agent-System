import sys
import requests
import xml.etree.ElementTree as ET
import json

def fetch_jobs(job_title, location):
    url = f"https://www.indeed.com/rss?q={job_title}&l={location}"
    response = requests.get(url)
    if response.status_code != 200:
        print(f"Failed to fetch jobs: {response.status_code}")
        return []
    
    root = ET.fromstring(response.content)
    jobs = []
    for item in root.findall('.//item'):
        title = item.find('title').text
        company = item.find('company').text
        location = item.find('location').text
        url = item.find('link').text
        date = item.find('pubDate').text
        jobs.append({
            'title': title,
            'company': company,
            'location': location,
            'url': url,
            'date': date
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
