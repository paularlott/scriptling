import time as _time
import utils


def index(request):
    return {"status": 200, "headers": {"Content-Type": "text/html"}, "body": utils.render_page()}


def current_time(request):
    return {"status": 200, "body": _time.time()}


def echo(request):
    return {"status": 200, "headers": {"Content-Type": "application/json"}, "body": request["body"]}
