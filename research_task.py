import requests

def fetch_trends():
    url = "http://localhost:11434"
    headers = {
        "Content-Type": "application/json",
        "Authorization": "Bearer YOUR_API_KEY"  # Replace with your actual API key if needed
    }
    payload = {
        "prompt": "What are the top 5 trends in LLMOps and AI engineering for 2026? Be specific and technical."
    }

    response = requests.post(url, headers=headers, json=payload)
    if response.status_code == 200:
        return response.json()
    else:
        raise Exception(f"Failed to fetch trends: {response.status_code} - {response.text}")

def save_output(output):
    with open("research_output.txt", "w") as file:
        file.write(str(output))

def print_summary(output):
    summary = "Top 5 Trends in LLMOps and AI Engineering for 2026:\n"
    for i, trend in enumerate(output.get("trends", []), start=1):
        summary += f"{i}. {trend}\n"
    print(summary)

def main():
    try:
        trends = fetch_trends()
        save_output(trends)
        print_summary(trends)
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    main()
