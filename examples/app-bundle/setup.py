import scriptling.runtime as runtime

# HTTP routes — handlers resolve from the bundle's lib/ dir.
runtime.http.get("/", "handlers.index")
runtime.http.get("/api/time", "handlers.current_time")
runtime.http.post("/api/echo", "handlers.echo")
