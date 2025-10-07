from opentelemetry import trace, metrics
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.semconv.resource import ResourceAttributes
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.requests import RequestsInstrumentor
try:
    from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
    HTTPX_AVAILABLE = True
except ImportError:
    HTTPX_AVAILABLE = False

from config import settings


def setup_telemetry(app):
    """Configure OpenTelemetry tracing and metrics."""
    
    # Create resource with service information
    resource = Resource(attributes={
        ResourceAttributes.SERVICE_NAME: settings.otel_service_name,
        ResourceAttributes.SERVICE_VERSION: settings.service_version,
    })
    
    # Setup tracing
    trace_provider = TracerProvider(resource=resource)
    trace_exporter = OTLPSpanExporter(
        endpoint=f"{settings.otel_exporter_otlp_endpoint}/v1/traces"
    )
    trace_provider.add_span_processor(BatchSpanProcessor(trace_exporter))
    trace.set_tracer_provider(trace_provider)
    
    # Setup metrics
    metric_exporter = OTLPMetricExporter(
        endpoint=f"{settings.otel_exporter_otlp_endpoint}/v1/metrics"
    )
    metric_reader = PeriodicExportingMetricReader(
        metric_exporter,
        export_interval_millis=10000,
    )
    meter_provider = MeterProvider(resource=resource, metric_readers=[metric_reader])
    metrics.set_meter_provider(meter_provider)
    
    # Instrument FastAPI
    FastAPIInstrumentor.instrument_app(app)
    
    # Instrument HTTP clients - this will automatically trace Flipt SDK calls!
    RequestsInstrumentor().instrument()
    if HTTPX_AVAILABLE:
        HTTPXClientInstrumentor().instrument()
    
    return trace.get_tracer(__name__), metrics.get_meter(__name__)
