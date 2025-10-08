import logging
from typing import Optional
import flipt
from opentelemetry import trace
from flipt.evaluation import EvaluationRequest

from config import settings

logger = logging.getLogger(__name__)


class FliptService:
    """Service for interacting with Flipt feature flags."""
    
    def __init__(self):
        self.client = None
        self.tracer = trace.get_tracer(__name__)
        self._initialize_client()
    
    def _initialize_client(self):
        """Initialize Flipt client."""
        try:
            self.client = flipt.FliptClient(
                url=settings.flipt_url,
            )
            logger.info(f"Flipt client initialized: {settings.flipt_url}")
        except Exception as e:
            logger.error(f"Failed to initialize Flipt client: {e}")
            self.client = None
    
    def evaluate_boolean(
        self, 
        flag_key: str, 
        entity_id: str, 
        context: dict = None,
        default: bool = False
    ) -> bool:
        """Evaluate a boolean flag."""
        with self.tracer.start_as_current_span(f"feature_flag.evaluation") as span:
            span.set_attribute("flag_feature.key", flag_key)
            span.set_attribute("flag_feature.type", "boolean")
            
            if not self.client:
                logger.warning(f"Flipt client not available, returning default for {flag_key}")
                return default
            
            try:
                # RequestsInstrumentor will automatically propagate trace context
                result = self.client.evaluation.boolean(EvaluationRequest(
                    namespace_key=settings.flipt_namespace,
                    flag_key=flag_key,
                    entity_id=entity_id,
                    context=context or {}
                ))
                enabled = result.enabled
                span.set_attribute("feature_flag.result.variant", enabled or "false")
                logger.debug(f"Flag '{flag_key}' evaluated to {enabled} (reason: {result.reason})")
                return enabled
            except Exception as e:
                logger.error(f"Error evaluating boolean flag '{flag_key}': {e}")
                span.set_attribute("error", True)
                span.set_attribute("error.message", str(e))
                return default
    
    def evaluate_variant(
        self, 
        flag_key: str, 
        entity_id: str, 
        context: dict = None,
        default: str = None
    ) -> Optional[str]:
        """Evaluate a variant flag and return the variant key."""
        with self.tracer.start_as_current_span(f"feature_flag.evaluation") as span:
            span.set_attribute("feature_flag.key", flag_key)
            span.set_attribute("feature_flag.type", "variant")
            
            if not self.client:
                logger.warning(f"Flipt client not available, returning default for {flag_key}")
                return default
            
            try:
                # RequestsInstrumentor will automatically propagate trace context
                result = self.client.evaluation.variant(EvaluationRequest(
                    namespace_key=settings.flipt_namespace,
                    flag_key=flag_key,
                    entity_id=entity_id,
                    context=context or {}
                ))
                variant_key = default
                if len(result.variant_key) > 0:
                    variant_key = result.variant_key
                span.set_attribute("feature_flag.result.variant", variant_key or "none")
                
                logger.debug(f"Flag '{flag_key}' evaluated to variant '{variant_key}' (reason: {result.reason})")
                return variant_key
            except Exception as e:
                logger.error(f"Error evaluating variant flag '{flag_key}': {e}")
                span.set_attribute("error", True)
                span.set_attribute("error.message", str(e))
                return default
    
    def get_price_display_strategy(self, entity_id: str, context: dict = None) -> str:
        """Get the price display strategy from feature flag."""
        strategy = self.evaluate_variant(
            flag_key="price-display-strategy",
            entity_id=entity_id,
            context=context,
            default="per-night"
        )
        return strategy or "per-night"
    
    def is_real_time_availability_enabled(self, entity_id: str, context: dict = None) -> bool:
        """Check if real-time availability is enabled."""
        return self.evaluate_boolean(
            flag_key="real-time-availability",
            entity_id=entity_id,
            context=context,
            default=True
        )
    
    def is_loyalty_program_enabled(self, entity_id: str, context: dict = None) -> bool:
        """Check if loyalty program is enabled."""
        return self.evaluate_boolean(
            flag_key="loyalty-program",
            entity_id=entity_id,
            context=context,
            default=False
        )
    
    def is_instant_booking_enabled(self, entity_id: str, context: dict = None) -> bool:
        """Check if instant booking is enabled."""
        return self.evaluate_boolean(
            flag_key="instant-booking",
            entity_id=entity_id,
            context=context,
            default=False
        )


# Global Flipt service instance
flipt_service = FliptService()
