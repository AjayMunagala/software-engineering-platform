# System Flow

## Version 1: Repository Scanner

```text
User selects repository path
  -> Validate path and access
  -> Traverse files (respect ignore rules)
  -> Classify files and detect languages
  -> Read supported manifests/configuration
  -> Detect frameworks, build tools, tests, Docker and CI/CD
  -> Aggregate factual metadata
  -> Emit JSON report and readable summary
```

The scanner must only read repository content. It must not call an LLM, modify files, execute project commands, or transmit data.

## Future investigation flow

```text
Issue -> Evidence collection -> Hypotheses -> Verification -> Minimal patch proposal -> Validation -> Explanation
```
