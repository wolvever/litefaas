from django.http import HttpResponse, JsonResponse
from django.urls import path

def healthz(_request):
    return HttpResponse("ok", content_type="text/plain; charset=utf-8")

def root(_request):
    return JsonResponse({"service": "blog", "framework": "django"})

urlpatterns = [
    path("healthz", healthz),
    path("", root),
]
