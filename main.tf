terraform {
  required_providers {
    adaptive = {
      source = "adaptive-scale/local/adaptive"
    }
  }
}

provider "adaptive" {}

resource "adaptive_resource" "document_test_123" {
  type          = "mongodb_atlas"
  name          = "doc1-test-02"
  uri          = "testsomething"
}
