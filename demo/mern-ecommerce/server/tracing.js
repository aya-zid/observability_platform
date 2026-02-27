'use strict';

const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { Resource } = require('@opentelemetry/resources');
const { SEMRESATTRS_SERVICE_NAME, SEMRESATTRS_SERVICE_VERSION } = require('@opentelemetry/semantic-conventions');

// Where to send traces — your otel-collector
const OTEL_ENDPOINT = process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'http://otel-collector:4317';
const TENANT_ID     = process.env.TENANT_ID || 'obs_test123';

const sdk = new NodeSDK({
  resource: new Resource({
    [SEMRESATTRS_SERVICE_NAME]:    'mern-ecommerce-server',
    [SEMRESATTRS_SERVICE_VERSION]: '1.0.0',
    'tenant.id':                   TENANT_ID,
    'deployment.environment':      'demo',
  }),
  traceExporter: new OTLPTraceExporter({
    url: OTEL_ENDPOINT,
    headers: {
      'X-Scope-OrgID': TENANT_ID,
    },
  }),
  instrumentations: [
    getNodeAutoInstrumentations({
      // Capture HTTP requests
      '@opentelemetry/instrumentation-http': {
        enabled: true,
      },
      // Capture Express routes
      '@opentelemetry/instrumentation-express': {
        enabled: true,
      },
      // Capture MongoDB queries
      '@opentelemetry/instrumentation-mongodb': {
        enabled: true,
        dbStatementSerializer: (operation) => operation,
      },
      // Disable noisy fs instrumentation
      '@opentelemetry/instrumentation-fs': {
        enabled: false,
      },
    }),
  ],
});

sdk.start();
console.log('✓ OpenTelemetry tracing started');
console.log(`✓ Sending traces to: ${OTEL_ENDPOINT}`);
console.log(`✓ Tenant: ${TENANT_ID}`);

// Graceful shutdown
process.on('SIGTERM', () => {
  sdk.shutdown()
    .then(() => console.log('Tracing shut down'))
    .catch((error) => console.error('Error shutting down tracing', error))
    .finally(() => process.exit(0));
});
