from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    """Application settings with environment variable support."""
    
    # Service settings
    service_name: str = "hotel-service"
    service_version: str = "1.0.0"
    
    # Flipt settings
    flipt_url: str = "http://flipt:8080"
    flipt_namespace: str = "default"
    flipt_environment: str = "onoffinc"
    
    # OpenTelemetry settings
    otel_exporter_otlp_endpoint: str = "http://jaeger:4318"
    otel_exporter_metric_endpoint: str = "http://prometheus:9090"
    otel_service_name: str = "hotel-service"
    
    # CORS settings
    cors_origins: list[str] = ["http://localhost:4000", "http://localhost:8080", "http://webapp"]
    
    class Config:
        env_file = ".env"
        case_sensitive = False


settings = Settings()
