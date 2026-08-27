import scriptling.runtime.http as http

# Demonstrates a handler module inside a subdirectory: routes/status.py is
# imported as "import routes.status" and registers via decorators at import
# time. The server resolves the handler as "routes.status.component_status".

@http.get("/api/status/{component}")
def component_status(request):
    component = request.path_param("component")
    statuses = {
        "api": "ok",
        "db": "ok",
        "cache": "degraded",
    }
    if component in statuses:
        return http.json(200, {"component": component, "status": statuses[component]})
    return http.json(404, {"error": f"unknown component: {component}"})
