import os
from datetime import datetime

def generate_report(results):
    model = results["model"]
    total_tests = results["total_tests"]
    passed = results["passed"]
    failed = results["failed"]
    blocked_by_guardrails = results["blocked_by_guardrails"]
    categories = results["categories"]
    timestamp = results["timestamp"]
    
    report_path = f"reports/eval_{model}_{datetime.now().strftime('%Y%m%d%H%M%S')}.md"
    
    with open(report_path, "w") as f:
        f.write(f"# Evaluation Report for {model}\n\n")
        f.write(f"**Timestamp:** {timestamp}\n\n")
        f.write(f"**Total Tests:** {total_tests}\n")
        f.write(f"**Passed:** {passed}\n")
        f.write(f"**Failed:** {failed}\n")
        f.write(f"**Blocked by Guardrails:** {blocked_by_guardrails}\n\n")
        
        f.write("## Category Scores\n\n")
        for category, score in categories.items():
            f.write(f"- **{category.capitalize()}**: {score}/{len([t for t in results['tests'] if t['category'] == category])}\n")
    
    with open(report_path, "r") as f:
        report_content = f.read()
    
    print(report_content)
