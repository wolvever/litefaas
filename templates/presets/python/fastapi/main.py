import os

from fastapi import FastAPI, Request
from fastapi.responses import PlainTextResponse, Response

app = FastAPI()


@app.get("/healthz")
def healthz():
    return PlainTextResponse("ok")


@app.api_route("/", methods=["GET", "POST"])
async def root(request: Request):
    body = await request.body()
    if not body:
        body = b'{"ok":true,"runtime":"python","preset":"fastapi"}'
    return Response(content=body, media_type="application/json")


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("PORT") or "8080")
    uvicorn.run(app, host="0.0.0.0", port=port)
