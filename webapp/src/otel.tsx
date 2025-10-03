import { metrics } from "@opentelemetry/api";
import { OTLPMetricExporter } from "@opentelemetry/exporter-metrics-otlp-http";
import {
  MeterProvider,
  PeriodicExportingMetricReader,
} from "@opentelemetry/sdk-metrics";

import { resourceFromAttributes } from "@opentelemetry/resources";
import { ATTR_SERVICE_NAME } from "@opentelemetry/semantic-conventions";

function getInstanceId() {
  const key = "flipt_demo_id";
  let id = localStorage.getItem(key);
  if (!id) {
    let agent = "unknown";
    if (navigator?.userAgent !== undefined) {
      const ua = navigator.userAgent.toLowerCase();
      if (
        ua.includes("chrome") &&
        !ua.includes("edge") &&
        !ua.includes("opr")
      ) {
        agent = "chrome";
      } else if (ua.includes("safari") && !ua.includes("chrome")) {
        agent = "safari";
      } else if (ua.includes("firefox")) {
        agent = "firefox";
      } else if (ua.includes("edg")) {
        agent = "edge";
      }
    }
    let random;
    if (crypto.getRandomValues !== undefined) {
      const arr = new Uint32Array(1);
      crypto.getRandomValues(arr);
      random = arr[0].toString(36);
    } else {
      random = Math.random().toString(36);
    }

    id = `${agent}-${random}`;
    localStorage.setItem(key, id);
  }
  return id;
}

const resource = resourceFromAttributes({
  [ATTR_SERVICE_NAME]: "flipt-client",
});

const meterProvider = new MeterProvider({
  resource: resource,
  readers: [
    new PeriodicExportingMetricReader({
      exporter: new OTLPMetricExporter({ url: "/internal/v1/metrics" }),
      exportIntervalMillis: 10000,
    }),
  ],
});
metrics.setGlobalMeterProvider(meterProvider);

const createHook = (environment: string, namespace: string) => {
  const meter = metrics.getMeter("flipt-client");
  const requestCounter = meter.createCounter(
    "flipt_evaluations_requests_total",
  );
  const resultsCounter = meter.createCounter("flipt_evaluations_results_total");
  const instanceId = getInstanceId();
  return {
    before: ({ flagKey }: { flagKey: string }) => {
      requestCounter.add(1, {
        flipt_flag: flagKey,
        flipt_environment: environment,
        flipt_namespace: namespace,
        instance: instanceId,
      });
    },
    after: ({
      flagKey,
      flagType,
      value,
      reason,
    }: {
      flagKey: string;
      flagType: string;
      value: string;
      reason: string;
    }) => {
      resultsCounter.add(1, {
        flipt_flag: flagKey,
        flipt_environment: environment,
        flipt_namespace: namespace,
        flipt_value: value,
        flipt_reason: reason,
        flipt_flag_type: flagType,
        instance: instanceId,
      });
    },
  };
};

export default createHook;
