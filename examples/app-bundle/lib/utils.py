def render_page():
    return """<!DOCTYPE html>
<html>
<head><link rel="stylesheet" href="/style.css"></head>
<body>
<h1>App Bundle</h1>
<p>Served from webroot/ inside the bundle zip.</p>
<p>Try: <a href="/api/time">/api/time</a></p>
</body>
</html>"""
