# Performance-benchmarking the TUI

This project contains micro- and production-style Go benchmarks that measure
how fast the Bubble Tea update-loop runs and how many allocations it performs.
It also ships a **message sampler** which lets you record the exact mix of
`tea.Msg` types observed during a real session and feed that back into a
benchmark.

## 1. Collect a production sample (optional)

1. Build **envctl** with the `msgsample` tag:

   ```bash
   go run -tags msgsample ./cmd/envctl <your-flags>
   ```

2. Use the application in a normal way for ~1-2 min: navigate a few panels,
   let the health ticker run, etc.

3. Quit with **q** **or** **Ctrl-C**.

   A file called `msg_sample.json` will be written to the current working
   directory. It contains a map from message-type to occurrence count, e.g.:

   ```json
   {
     "spinner.TickMsg": 408,
     "tea.KeyMsg": 4,
     "tea.MouseMsg": 15,
     "tui.portForwardCoreUpdateMsg": 5,
     "tui.requestClusterHealthUpdate": 1
   }
   ```

4. (Optionally) copy that JSON into the repository and adjust
   `internal/tui/model_update_bench_prod_test.go` so the benchmark reflects your
   latest workload.

## 2. Running the benchmarks

### Quick micro-benchmark

Measures worst-case cost (1 000 messages with lots of resizes).

```bash
go test ./internal/tui -bench=BenchmarkModelUpdate -benchmem
```

### Production-mix benchmark

Replays the burst created from `msg_sample.json`:

```bash
go test ./internal/tui -bench=BenchmarkModelUpdateProduction -benchmem
```

### Capture a baseline for CI

Run each benchmark six times to get statistically useful samples and save the
result:

```bash
# first run after making changes
go test ./internal/tui -bench=BenchmarkModelUpdateProduction -benchmem -count=6 > bench-new.txt

# compare against a stored baseline
benchstat bench-base.txt bench-new.txt
```

## 3. Integrating into CI

1. Check-in an initial baseline file (e.g. `bench-base.txt`).
2. Add a CI step that runs the benchmarks and compares with `benchstat`.
3. Fail the pipeline when `sec/op`, `B/op` or `allocs/op` regress beyond an
   agreed threshold (e.g. +15 %).

---

### Troubleshooting

*"msg_sample.json doesn't appear"*  → You must build with `-tags msgsample` **and**
exit via **q** or **Ctrl-C**; both paths call `finalizeMsgSampling()` which writes
 the file.

*Bench output is cluttered with `=== Test` lines*  → Remove old experimental
 test files or ensure they use `t.Helper()`/proper test naming so they don't
 print during `go test`. 