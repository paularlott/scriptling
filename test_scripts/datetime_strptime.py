import datetime

date_str = "2024-01-15 10:30:45"
timestamp = datetime.strptime(date_str, "%Y-%m-%d %H:%M:%S")
len(str(timestamp)) > 0