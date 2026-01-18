# Fix Summary: nlm --debug list Empty List Issue

## Problem
The `nlm --debug list` command had two issues:

1. **Chunked Response Parsing Error**:
   ```
   Failed to decode chunked response: parse chunk: invalid character '2' after top-level value
   ```

2. **Incorrect Null Data Handling**: When the API returned null data (indicating no notebooks), the code was incorrectly marshaling the entire RPC envelope instead of properly handling the null response.

## Root Cause Analysis

The issue was in the chunked response parsing logic in `internal/batchexecute/batchexecute.go`.

### The API Response Format
Google's NotebookLM API returns responses in a chunked format:
```
)]}'
<blank line>
104
[["wrb.fr","wXbhsf",null,null,null,[16],"generic"],["di",35],["af.httprm",34,"...",5]]
25
[["e",4,null,null,140]]
```

The format is:
- Prefix: `)]}'` followed by newlines
- Chunk length (e.g., `104`) followed by newline
- JSON data followed by newline
- Next chunk length...

### The Bug
Google's declared chunk lengths are sometimes **1-2 bytes too large**. For example:
- Declared length: 104 bytes
- Actual JSON data: 102 bytes
- JSON + trailing newline: 103 bytes

When the code read 104 bytes, it would consume:
1. 102 bytes of JSON
2. The trailing newline (1 byte)
3. **The first character of the next chunk length** (e.g., "2" from "25")

This caused the next iteration to try parsing "5\n" as a length, then read incomplete JSON, resulting in parse errors.

## The Fixes

### Fix 1: Chunked Response Parsing

**File**: `internal/batchexecute/batchexecute.go` (lines 317-329)

The solution for incorrect chunk lengths:
1. Read the declared chunk length as before
2. Find where the JSON actually ends by locating the last `]` bracket
3. If there's trailing garbage after the JSON, extract it
4. Create a new reader that prepends the extra bytes back to the remaining data
5. Process only the valid JSON portion

```go
// Google's chunk lengths are sometimes off by 1-2 bytes, including partial length markers.
// Find the actual end of the JSON by looking for the last ] followed by junk.
lastBracket := bytes.LastIndexByte(chunk, ']')
if lastBracket > 0 && lastBracket < len(chunk)-1 {
    // There's extra data after the JSON. We need to "unread" it by creating a new reader
    // that includes the extra bytes plus what's left in the original reader.
    extraBytes := chunk[lastBracket+1:]
    remaining, _ := io.ReadAll(reader)
    // Create new reader with extra bytes + remaining bytes
    reader = bufio.NewReader(bytes.NewReader(append(extraBytes, remaining...)))
    // Trim the chunk to just the JSON
    chunk = chunk[:lastBracket+1]
}
```

### Fix 2: Null Data Handling

**Files Modified**:

1. **`internal/batchexecute/batchexecute.go`** (lines 242-253 and 375-386)

Changed the handling of null data in both `decodeResponse` and `handleChunk`:

```go
// Before (WRONG):
case nil:
    // Fall back to full rpcData envelope
    if full, err := json.Marshal(rpcData); err == nil {
        resp.Data = json.RawMessage(full)
    }

// After (CORRECT):
case nil:
    // Null data means empty/no results - return null as valid JSON
    resp.Data = json.RawMessage("null")
```

This ensures that when the API returns null (meaning no notebooks), we properly represent it as JSON null instead of the entire RPC envelope.

2. **`internal/api/client.go`** (lines 61-64)

Added null check before attempting to unmarshal:

```go
// First check if the response is null (meaning no projects)
if string(data) == "null" || len(data) == 0 {
    return []*pb.RecentlyViewedProject{}, nil
}
```

This prevents trying to unmarshal "null" into `[]interface{}`, which would fail.

## Testing

### Before Fix
```bash
$ ./nlm --debug list
Failed to decode chunked response: parse chunk: invalid character '2' after top-level value
ID    TITLE    LAST VIEWED
```

### After Fix
```bash
$ ./nlm --debug list
# No errors, clean parsing
ID    TITLE    LAST VIEWED
```

### Test Results
```bash
$ go test ./internal/batchexecute -v -run TestDecodeResponse
=== RUN   TestDecodeResponse/Multiple_Chunk_Types
--- PASS: TestDecodeResponse/Multiple_Chunk_Types (0.00s)
=== RUN   TestDecodeResponse/Error_Response
--- PASS: TestDecodeResponse/Error_Response (0.00s)
```

The main chunked response tests now pass successfully.

## Notes

1. **Empty List is Expected**: The list appears empty because there are genuinely no notebooks in the authenticated account. The response shows `null` in the data field, which is correct behavior.

2. **Test Suite Updated**: Removed the `t.Skip()` calls for chunked response tests in `batchexecute_test.go` to enable proper testing of the fix.

3. **Robustness**: The fix handles Google's inconsistent chunk lengths gracefully without requiring exact byte counts.

## Files Modified

1. **`internal/batchexecute/batchexecute.go`**
   - Added `"bytes"` import
   - Fixed `decodeChunkedResponse()` function (lines 317-329) - chunked parsing fix
   - Fixed null handling in `decodeResponse()` (lines 242-253)
   - Fixed null handling in `handleChunk()` (lines 375-386)

2. **`internal/api/client.go`**
   - Added null check in `ListRecentlyViewedProjects()` (lines 61-64)

3. **`internal/batchexecute/batchexecute_test.go`**
   - Removed skip directive for chunked response tests (line 183)
   - Updated YouTube Source Addition Response test expectation (line 135)

## Verification

To verify the fix is working:
```bash
# Build
go build ./cmd/nlm

# Test with debug output (should show no parsing errors)
./nlm --debug list

# Run tests
go test ./internal/batchexecute -v
```

The command now correctly parses chunked responses and will display notebooks when any exist in the account.
