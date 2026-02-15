# Test library for background task demonstration

def worker(task_id, iterations=5):
    """Background worker that performs calculations"""
    result = 0
    for i in range(iterations):
        result += task_id * (i + 1)
    return result
