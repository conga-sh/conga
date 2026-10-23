# Kitchen Sink - HCL Format
# This file demonstrates every supported type and configuration pattern

# Root flags (top-level keys)
verbosity = 9
format = "json"
cache = true
api_secret = "hcl-secret-key-xyz789"

# Feature toggles (root Features container -> features block)
features {
  http3       = false
  compression = 1
}

# Primitives subcommand
types {
  primitives {
    string_flag = "hcl-parsed"
    bool_flag = true
    int_flag = 400
    int8_flag = 32
    int16_flag = 640
    int32_flag = 12800
    int64_flag = 256000
    uint_flag = 800
    uint8_flag = 200
    uint16_flag = 6400
    uint32_flag = 128000
    uint64_flag = 256000
    float32_flag = 4.5
    float64_flag = 12.56637061436
    tags = ["hcl", "test", "validated"]
    ports = [7000, 7001, 7002]
  }

  # Filesystem subcommand
  filesystem {
    input_file = "/etc/hostname"
    output_dir = "/var/tmp"
    log_file = "/var/log/hcl-demo.log"
    data_dir = "/var/lib/hcl-demo"
    editor = "vi"
    hook = "deploy-hook"
  }

  # Value subcommand
  values {
    timeout = "4m30s"
    endpoint = "https://hcl.example.com/v4"
    max_size = "300MB"
    level = "info"

    shard_map = {
      web = "8"
      worker = "2"
    }
  }
}

# Config subcommand (shared context flag)
config {
  context = "hcl-context"

  # Config maps subcommand: native map vs array-of-strings encodings
  maps {
    vars = {
      WORKERS     = "10"
      BUFFER_SIZE = "4096"
      LOG_FORMAT  = "json"
    }
    labels = ["tier=worker", "team=compute", "version=4.2"]
  }

  # Config nested subcommand (deep nesting)
  nested {
    zone = "ca-central-1a"
    count = 15
  }
}
