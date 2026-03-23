# WebSocket Server Handlers
import scriptling.runtime as runtime

def home(request):
    """Simple HTTP endpoint that returns info about the server."""
    return runtime.http.html(200, """
    <html>
    <head><title>WebSocket Echo Server</title></head>
    <body>
        <h1>WebSocket Echo Server</h1>
        <p>Connect to ws://localhost:8080/echo to test the echo server.</p>
        <script>
            const ws = new WebSocket('ws://localhost:8080/echo');
            ws.onopen = () => {
                console.log('Connected!');
                ws.send('Hello from browser!');
            };
            ws.onmessage = (event) => {
                console.log('Received:', event.data);
            };
        </script>
    </body>
    </html>
    """)

def echo_handler(client):
    """WebSocket echo handler - echoes all messages back to the client."""
    client.send("Welcome to the echo server!")

    while client.connected():
        msg = client.receive(timeout=60)
        if msg:
            # Echo the message back
            client.send(f"Echo: {msg}")

    # Connection closed
    print(f"Client disconnected from {client.remote_addr}")
