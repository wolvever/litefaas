import os
from typing import Optional

from fastapi import FastAPI
from fastapi.responses import JSONResponse, PlainTextResponse
from sqlalchemy import Column, Integer, String, create_engine, select
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

app = FastAPI(title="catalog")


class Base(DeclarativeBase):
    pass


class Product(Base):
    __tablename__ = "products"

    id = Column(Integer, primary_key=True)
    sku = Column(String(64), unique=True, nullable=False)
    title = Column(String(255), nullable=False)


def session_factory() -> Optional[sessionmaker]:
    url = os.environ.get("DATABASE_URL")
    if not url:
        return None
    engine = create_engine(url, pool_pre_ping=True)
    Base.metadata.create_all(engine)
    return sessionmaker(bind=engine)


SessionLocal = session_factory()

SEED = [
    {"sku": "book-1", "title": "RFC-0001 annotated"},
    {"sku": "sticker", "title": "HTTP $PORT sticker"},
]


@app.get("/healthz")
def healthz():
    return PlainTextResponse("ok")


@app.get("/")
def root():
    if SessionLocal is None:
        return JSONResponse({"service": "catalog", "items": SEED, "store": "memory"})
    with SessionLocal() as db:  # type: Session
        rows = db.scalars(select(Product)).all()
        if not rows:
            for item in SEED:
                db.add(Product(sku=item["sku"], title=item["title"]))
            db.commit()
            rows = db.scalars(select(Product)).all()
        return JSONResponse({
            "service": "catalog",
            "store": "sql",
            "items": [{"sku": r.sku, "title": r.title} for r in rows],
        })


if __name__ == "__main__":
    import uvicorn

    port = int(os.environ.get("PORT") or "8080")
    uvicorn.run(app, host="0.0.0.0", port=port)
