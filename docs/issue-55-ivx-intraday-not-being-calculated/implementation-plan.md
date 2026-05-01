# Issue 55: IVx Intraday Not Being Calculated - Implementation Plan

## Step 1: Fix the timezone bug in calculator.go

Update the IVx calculator to use `America/New_York` (Eastern Time) when determining market hours and whether intraday calculations should run. The current code uses UTC, which causes intraday IVx calculations to be skipped outside of UTC market hours.

## Step 2: Add unit tests in calculator_test.go

Add unit tests covering the timezone-sensitive logic to ensure intraday IVx calculations are triggered correctly during Eastern Time market hours, regardless of the server's local timezone.

## Step 3: Run all tests to verify no regressions

Run the full backend test suite (`go test ./...`) to confirm the fix doesn't break existing functionality.
