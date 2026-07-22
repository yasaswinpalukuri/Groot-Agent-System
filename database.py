import sqlite3

def create_db():
    conn = sqlite3.connect('jobs.db')
    cursor = conn.cursor()
    
    # Create table
    cursor.execute('''
        CREATE TABLE IF NOT EXISTS jobs (
            id INTEGER PRIMARY KEY,
            company TEXT,
            role TEXT,
            location TEXT,
            salary TEXT,
            status TEXT,
            date_applied TEXT,
            score INTEGER
        )
    ''')
    
    # Insert sample data
    sample_data = [
        ('Shopify', 'Data Engineer', 'Toronto, ON', '$100K - $120K', 'Applied', '2023-04-01', 85),
        ('RBC', 'Data Engineer', 'Vancouver, BC', '$90K - $110K', 'Pending', '2023-04-02', 78),
        ('Wealthsimple', 'Data Engineer', 'Toronto, ON', '$110K - $130K', 'Interviewing', '2023-04-03', 92),
        ('TD Bank', 'Data Engineer', 'Montreal, QC', '$85K - $105K', 'Rejected', '2023-04-04', 67),
        ('Hootsuite', 'Data Engineer', 'Vancouver, BC', '$95K - $115K', 'Accepted', '2023-04-05', 88),
        ('Cohere', 'Data Engineer', 'Toronto, ON', '$120K - $140K', 'Applied', '2023-04-06', 79),
        ('Layer6', 'Data Engineer', 'Vancouver, BC', '$105K - $125K', 'Pending', '2023-04-07', 82),
        ('Bell', 'Data Engineer', 'Toronto, ON', '$115K - $135K', 'Interviewing', '2023-04-08', 90),
        ('Telus', 'Data Engineer', 'Vancouver, BC', '$90K - $110K', 'Rejected', '2023-04-09', 65),
        ('FreshBooks', 'Data Engineer', 'Toronto, ON', '$100K - $120K', 'Accepted', '2023-04-10', 87)
    ]
    
    cursor.executemany('''
        INSERT INTO jobs (company, role, location, salary, status, date_applied, score)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    ''', sample_data)
    
    conn.commit()
    conn.close()

def main():
    create_db()
    print('Database ready')

if __name__ == '__main__':
    main()
