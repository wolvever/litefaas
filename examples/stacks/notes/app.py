import os
from flask import Flask, jsonify
from flask_sqlalchemy import SQLAlchemy
from sqlalchemy import select

app = Flask("notes")
db = SQLAlchemy()

SEED = [
    {"slug": "hello", "body": "zero-config flask notes"},
    {"slug": "port", "body": "bind 0.0.0.0:$PORT"},
]


class Note(db.Model):
    __tablename__ = "notes"

    id = db.Column(db.Integer, primary_key=True)
    slug = db.Column(db.String(64), unique=True, nullable=False)
    body = db.Column(db.String(255), nullable=False)


def configure() -> None:
    url = os.environ.get("DATABASE_URL")
    if not url:
        app.config["SQLALCHEMY_DATABASE_URI"] = "sqlite:///:memory:"
        app.config["_STORE"] = "memory"
    else:
        app.config["SQLALCHEMY_DATABASE_URI"] = url
        app.config["_STORE"] = "sql"
    app.config["SQLALCHEMY_TRACK_MODIFICATIONS"] = False
    db.init_app(app)
    with app.app_context():
        db.create_all()
        if db.session.scalar(select(Note).limit(1)) is None:
            for item in SEED:
                db.session.add(Note(slug=item["slug"], body=item["body"]))
            db.session.commit()


configure()


@app.get("/healthz")
def healthz():
    return "ok", 200, {"Content-Type": "text/plain; charset=utf-8"}


@app.get("/")
def root():
    rows = db.session.scalars(select(Note)).all()
    return jsonify(
        {
            "service": "notes",
            "store": app.config.get("_STORE", "memory"),
            "items": [{"slug": r.slug, "body": r.body} for r in rows],
        }
    )


if __name__ == "__main__":
    port = int(os.environ.get("PORT") or "8080")
    app.run(host="0.0.0.0", port=port)
