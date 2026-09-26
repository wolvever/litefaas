import os

from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse, PlainTextResponse

app = FastAPI()


@app.get("/healthz")
def healthz():
    return PlainTextResponse("ok")


@app.api_route("/", methods=["GET", "POST"])
async def root(request: Request):
    who = "{{name}}"
    try:
        data = await request.json()
        if isinstance(data, dict) and data.get("name"):
            who = str(data["name"])
    except Exception:
        pass
    return JSONResponse({"message": "hello from " + who, "function": "{{name}}"})


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("PORT") or "8080")
    uvicorn.run(app, host="0.0.0.0", port=port)
