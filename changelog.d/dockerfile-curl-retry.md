- **`hct dockerfile`:** the generated Dockerfile gives curl a config file
  (through `CURL_HOME`) with the installers' retry policy plus
  `--retry-all-errors`, so a transient download failure, such as a dropped
  TLS connection to api.adoptium.net, is retried instead of failing the
  build. It applies to every `curl` in the build and in the image.
