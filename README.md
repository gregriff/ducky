# gpt-cli-go

---

## Overview

This project is a rewrite of the GPT-CLI Python repository. It aims to use a better client-server architecture to reduce CPU utilization when multiple (now client) instances are run, allow better configuration with [Viper](https://github.com/spf13/viper) and provide more features (but probably less customizability!) than the previous project.

I'm doing this rewrite mainly to learn Go and get experience with its concurrency patterns.
