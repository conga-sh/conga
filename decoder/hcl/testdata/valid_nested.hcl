tags = ["prod", "v2"]

cluster {
  region = "us-east-1"

  node {
    zone = "us-east-1a"

    config {
      timeout = 30
      retries = 3
    }
  }
}
