---
name: code-review
description: General code review command for white-box logic correctness review and technical architecture evaluation based on business requirements
---

# Code Review Command

## Review Task

Please conduct a comprehensive review of code changes in the current workspace, focusing on the correctness of business logic and the rationality of technical architecture.

## Review Dimensions

### 1. Business-Level Review (BLOCKING Level)
- **Requirement Source Verification**: Find and verify business requirement sources (cannot infer requirements from code)
- **Requirement Implementation Completeness**: Verify that code 100% satisfies business requirements
- **Business Process Logic Correctness**: Check logic of state transitions, data flow, and conditional judgments
- **Data Structure Correctness**: Verify model definitions, field constraints, and business rule mappings
- **Edge Case Handling**: Evaluate the reasonableness of handling boundary conditions and exception scenarios

### 2. Technical-Level Review
- **Architecture Rationality**: Modularity, separation of concerns, dependency relationships
- **KISS Principle Adherence**: Avoid over-engineering
- **Extensibility Assessment**: Ease of adding future features
- **Non-Adhoc Modification Verification**: Whether existing code patterns are followed
- **Performance Issue Detection**: Find obvious performance issues (such as N+1 queries, etc.)
- **Unit Test Completeness**: 90% coverage requirement for core logic

### 3. Contract and Connectivity Special Check (BLOCKING)
- Endpoint Consistency: Frontend endpoints centrally configured; paths/dynamic segments/case/trailing separators completely match backend routes; HTTP method semantics match (idempotent/side effects).
- Authentication and CORS: Unified authentication mechanism (session/token); frontend passing method matches backend expectations (Cookie/Authorization, etc.); CORS and CSRF policies match.
- Request/Response Schema: Field names, types, required/optional consistent; time/numeric/boolean encoding consistent; pagination/sorting parameters align with response metadata.
- Errors and Status Codes: 4xx/5xx usage reasonable; error payload structure stable and parseable; frontend provides recoverable prompts based on error types.
- Async/Events (if applicable): Event types and payload fields match documentation; semantics complete (start/end/error/heartbeat); incremental merge without loss/duplication; fallback path exists.
- End-to-End Propagation: API→Service→Downstream (storage/third-party) parameters (context/session/region/idempotency key) not missing; write operations have idempotency/deduplication strategy.
- Logging and Privacy: Logs contain necessary context (trace/user/session), sensitive information masked; UI does not display internal implementation details by default.

#### Full-Chain Contract Crosswalk (MANDATORY)
- Select key user flows, establish "parameter alignment matrix":
  - Frontend request → Backend route/handler → Service layer → Downstream (storage/third-party) → Persisted fields.
  - List each parameter: `name | type | required | default | source-of-truth(file:line)`.
  - If event/streaming contracts exist, supplement event types and payload fields, as well as necessary state writebacks.
- Output discrepancies and minimal fix suggestions (who changes, where, how to not break existing behavior).

### 4. High-Frequency Issue Checklist (Priority Investigation)
- Naming Mismatch: Different layers use different naming styles or names for the same concept (e.g., `camelCase` ↔ `snake_case`, identifier naming inconsistent).
- Path Mismatch: Endpoint paths or dynamic segment naming inconsistent; trailing separators cause redirects or method failures.
- Type Drift: Number/string/boolean/time types inconsistently encoded or incorrectly converted across layers.
- Authentication Mixing: Requests use different authentication methods in different places; cross-origin requests don't carry credentials as needed; CSRF protection missing.
- Event Gaps: Backend adds new event types or fields not parsed by frontend; incremental merge logic causes duplicates/loss.
- Visibility/State: Backend state bits or visibility fields ignored, causing UI display inconsistent with actual state.

### 5. Zero-Assumption Verification (VERIFY-FIRST Gate, BLOCKING)
- Prohibit writing logic based on "guessed fields/APIs/events". Each critical element must provide evidence:
  - Model/field definition location (file:line)
  - Route/handler method signature (file:line)
  - Frontend type/parsing/call code (file:line)
- Assumptions without evidence are not allowed.

### 6. User Perspective E2E Visibility Audit (BLOCKING)
⚠️ **This is the most critical check** - The main reason "code is perfect but feature doesn't work" is skipping this step!

**MANDATORY USER FLOW VERIFICATION (Must Execute):**
- **Complete Click Path Tracking**: From user click, step-by-step track to final state
  - User clicks X → Calls function Y → Navigates to page Z → Displays content W
  - **Must verify each step executes correctly**
- **URL Route Verification**: All navigation paths exist in route configuration and correctly handle parameters
- **State Transfer Verification**: Whether state changes after click are correctly reflected in UI
- **Error Scenario Testing**: Handling of missing parameters, network errors, insufficient permissions, etc.

**Specific Check Items:**
- Entry Visibility: Entry points (buttons/navigation/controls) for corresponding features are reachable in default scenarios and target devices; not hidden by misjudged conditions.
- **Link Landing (Core)**: Page routes/back navigation/deep links consistent; forms closed loop from entry to completion.
  - Click notification → Does it actually jump to expected page?
  - Share link → Does it actually load expected content?
  - All navigation paths must be actually tracked and verified!
- State Completeness: Loading/empty data/error/insufficient permissions all have clear presentation and recoverable paths.
- Role/Switch: Visibility matches permissions/Feature Flag expectations; default values don't block main flow.

**⛔ Prohibited Behaviors:**
- ❌ Only look at code structure, don't track actual execution flow
- ❌ Assume navigate() call equals user reaching target page
- ❌ Don't verify URL parameter handling logic
- ❌ Say "looks correct" without verifying "actually correct"

### 7. Web3 AI Trading System Security Review (BLOCKING Level)
🔐 **Fund Security is the Bottom Line** - One security vulnerability can lead to total fund loss!

#### 7.1 Private Key and Key Management (CRITICAL)
- **Zero Leakage Principle**:
  - ❌ Prohibited: Private keys/mnemonics appear in logs, error messages, frontend code, Git history
  - ❌ Prohibited: Plaintext storage of private keys (environment variables, config files, database)
  - ✅ Required: Use hardware wallets, HSM, or encrypted key management services (AWS KMS/Vault)
  - ✅ Required: API Keys, key material must be encrypted at rest, decrypted at runtime
- **Principle of Least Privilege**:
  - Separate trading signature keys from read-only query keys
  - Each feature uses independent sub-accounts/permissions
  - Regularly rotate API Keys and access tokens
- **Verification Checks**:
  - [ ] grep search for `private_key`, `mnemonic`, `seed` and other keywords, ensure no hardcoding
  - [ ] Check encryption status of all key storage locations
  - [ ] Verify key access logs and audit trails

#### 7.2 Trading Security (CRITICAL)
- **Signature Verification**:
  - ✅ Required: All transactions must pass signature verification
  - ✅ Required: Verify transaction initiator identity (prevent forgery)
  - ✅ Required: Use nonce/sequence numbers to prevent replay attacks
- **Transaction Parameter Validation**:
  - ✅ Required: Verify recipient address legitimacy (checksum, whitelist)
  - ✅ Required: Amount/price/slippage limits (prevent abnormal large transactions)
  - ✅ Required: Gas Price/Gas Limit upper bound protection (prevent gas exhaustion attacks)
  - ✅ Required: Deadline/timeout protection (prevent expired transaction execution)
- **Slippage and Price Protection**:
  - ✅ Required: Set reasonable slippage tolerance (e.g., 0.5%-2%)
  - ✅ Required: Price oracle verification (multi-source comparison, timestamp check)
  - ✅ Required: Reject transactions on abnormal price volatility
- **Verification Checks**:
  - [ ] All transaction calls have nonce or idempotency key
  - [ ] Amount/price parameters have upper and lower bound validation
  - [ ] Gas fees have maximum limit
  - [ ] Slippage protection code exists and is correct

#### 7.3 AI Decision Security (CRITICAL)
- **Prompt Injection Protection**:
  - ❌ Prohibited: Directly concatenate user input into AI prompt
  - ✅ Required: Sanitize/escape user input (prevent prompt injection)
  - ✅ Required: Clearly separate system prompts from user input (use role isolation)
  - ✅ Required: Sensitive operations require explicit user confirmation, AI cannot autonomously decide large transactions
- **Decision Auditing**:
  - ✅ Required: Record complete context of all AI decisions (input, output, timestamp, model version)
  - ✅ Required: Decisions are traceable, replayable, auditable
  - ✅ Required: Alert on abnormal decisions (e.g., sudden large transaction suggestions)
- **Model Security**:
  - ✅ Required: Use official APIs, avoid third-party proxies (prevent man-in-the-middle attacks)
  - ✅ Required: API response validation (detect abnormal output, format errors)
  - ✅ Required: Model output not directly executed, must pass parameter validation
- **Verification Checks**:
  - [ ] Search user input concatenation points, ensure sanitization
  - [ ] Check decision logs are complete (include all critical parameters)
  - [ ] Verify large transactions require additional confirmation mechanism

#### 7.4 Smart Contract Interaction Security (CRITICAL)
- **Authorization Scope Control**:
  - ❌ Prohibited: Unlimited authorization (`approve(spender, type(uint256).max)`)
  - ✅ Required: Authorize on demand, calculate precise authorization amount before each transaction
  - ✅ Required: Regularly clean up expired authorizations
  - ✅ Required: Monitor authorization events, alert on abnormal authorizations
- **Contract Call Verification**:
  - ✅ Required: Contract address whitelist (only interact with audited contracts)
  - ✅ Required: Function selector verification (prevent calling wrong function)
  - ✅ Required: Call parameter type/range validation
  - ✅ Required: Simulate execution (dry-run) before real execution
- **Reentrancy and Exception Handling**:
  - ✅ Required: Handle contract call failures (revert, out of gas)
  - ✅ Required: Check return values, don't assume call succeeded
  - ✅ Required: Avoid modifying critical state after external calls (prevent reentrancy)
- **Verification Checks**:
  - [ ] grep `approve` to ensure no unlimited authorization
  - [ ] All contract addresses from config/whitelist, no hardcoding
  - [ ] Call failures have complete error handling and fallback logic

#### 7.5 Fund Protection Mechanisms (BLOCKING)
- **Limit Controls**:
  - ✅ Required: Single transaction amount upper limit (e.g., $1000)
  - ✅ Required: Daily/weekly/monthly cumulative limits
  - ✅ Required: Abnormal transaction frequency limits (prevent rapid fund depletion)
  - ✅ Required: Large transactions require multi-signature or delayed execution
- **Emergency Pause**:
  - ✅ Required: Global emergency stop button (kill switch)
  - ✅ Required: Automatic pause on anomaly detection (e.g., price anomaly, gas fee surge)
  - ✅ Required: Safe fund withdrawal mechanism after pause
- **Balance Monitoring**:
  - ✅ Required: Real-time balance monitoring, alert below threshold
  - ✅ Required: Alert on abnormal fund outflows (large transfers, unknown recipients)
  - ✅ Required: Regular reconciliation (on-chain balance vs system records)
- **Verification Checks**:
  - [ ] Limit configuration exists and is reasonable
  - [ ] Emergency pause function is testable and has permission control
  - [ ] Balance monitoring code exists and is connected to alert system

#### 7.6 On-Chain Data Validation (CRITICAL)
- **Oracle Security**:
  - ❌ Prohibited: Single data source (can be manipulated)
  - ✅ Required: Multi-oracle comparison (Chainlink, Band, UMA, etc.)
  - ✅ Required: Price deviation detection (reject if multi-source price difference exceeds threshold)
  - ✅ Required: Timestamp verification (data freshness check, reject stale data)
- **Block Confirmation**:
  - ✅ Required: Wait for sufficient block confirmations (mainnet recommends ≥12 blocks, L2 according to actual situation)
  - ✅ Required: Handle chain reorganization possibility (pending → confirmed → finalized)
  - ✅ Required: Transaction receipt verification (status=1 success)
- **Data Integrity**:
  - ✅ Required: Event log integrity check (topic, parameter matching)
  - ✅ Required: Contract state consistency verification (on-chain vs local cache)
  - ✅ Required: MEV protection (use private mempool or Flashbots)
- **Verification Checks**:
  - [ ] Price data from multiple oracles
  - [ ] Block confirmation count configured reasonably
  - [ ] Transaction status check includes finalized state

## Review Results

Please provide one of the following three results:
- ✅ **Pass**: Can be submitted directly
- ❌ **Fail**: BLOCKING issues exist, must be fixed
- ⚠️ **Needs Fix**: Room for improvement, recommended to fix

## Core Principles

1. **White-Box Logic Correctness is Fundamental**: Business logic errors are the bottom line
2. **Requirement-Driven**: Must find real requirement sources
3. **Objective Analysis**: Based on actual code and requirements, no self-deception
4. **Actionable Suggestions**: Provide specific fix guidance

## Review Deliverables (Must Include)
- **Issue List**: Point out "who and who are inconsistent" (paths/parameters/fields/events/status codes) item by item, with minimal reproduction samples.
- **Minimal Fix Suggestions**: Clearly state "who changes, where, how to not break existing calls" (can include 1-3 line-level diff suggestions).
- **Compatibility/Transition Strategy**: When necessary, explain dual parsing/version prefix/feature flag/fallback solutions.
- **🚨 E2E Verification Report**: Complete tracking verification for each user interaction flow (MANDATORY)

## Mandatory E2E Verification Checklist (Must Check Item by Item)
Before providing review results, the following verifications must be completed:

### ✅ User Click Verification
- [ ] All onClick handlers can execute correctly
- [ ] navigate() calls in handlers point to correct paths
- [ ] Target paths exist in route configuration
- [ ] Target pages can correctly handle URL parameters

### ✅ Navigation Flow Verification
- [ ] Complete path from click to page load is unobstructed
- [ ] URL parameters correctly passed and parsed
- [ ] Page state correctly initialized
- [ ] User sees expected content and interface

### ✅ State Consistency Verification
- [ ] Application state correctly updated after click
- [ ] UI interface reflects state changes
- [ ] No state synchronization issues

### ✅ Security Verification (Web3 AI Trading System - MANDATORY)
- [ ] **Key Security**: No private key leakage (logs/errors/frontend/Git)
- [ ] **Key Management**: Private keys encrypted at rest, no plaintext environment variables
- [ ] **Transaction Verification**: All transactions have signature verification, nonce, amount limits
- [ ] **Slippage Protection**: Price/slippage validation exists and is reasonable
- [ ] **AI Security**: User input has sanitization, not directly concatenated to prompt
- [ ] **Decision Auditing**: AI decisions have complete logs (input/output/timestamp)
- [ ] **Contract Security**: No unlimited authorization, contract addresses from whitelist
- [ ] **Limit Protection**: Single/cumulative transaction limits exist
- [ ] **Emergency Mechanism**: Has kill switch or pause function
- [ ] **Oracle Security**: Price data from multiple sources, has deviation detection
- [ ] **Confirmation Mechanism**: Large transactions require explicit user confirmation

### ⛔ Review Failure Conditions
If any of the following is true, the review must be marked as ❌ Fail:

**Functional Issues:**
- navigate() points to non-existent or incorrect paths
- User cannot reach expected page after click
- Incomplete state updates cause UI inconsistency
- Critical user flows cannot be completed

**Security Issues (Web3 AI System):**
- Private keys/mnemonics appear in logs, error messages, frontend code, Git history
- Private keys stored in plaintext (environment variables/config files/database)
- Transactions missing signature verification, nonce, or amount limits
- Unlimited authorization exists (`approve(spender, type(uint256).max)`)
- User input directly concatenated to AI prompt (prompt injection risk)
- AI can autonomously decide large transactions (no user confirmation)
- Emergency pause mechanism missing
- Single oracle data source (can be manipulated)
- Large transactions without multi-signature or delayed execution

**Remember: Code Compiles ≠ Function Works Correctly ≠ Funds Are Secure**

## Technical Verification Methods (MANDATORY)

### 🔍 Navigation Path Verification Script
Execute the following checks to verify navigation logic:
```bash
# 1. Find all navigate() calls
grep -r "navigate(" frontend/src --include="*.tsx" --include="*.ts" -n

# 2. Find all route definitions
grep -r "path=" frontend/src --include="*.tsx" --include="*.ts" -n

# 3. Check URL parameter handling
grep -r "useSearchParams\|URLSearchParams" frontend/src --include="*.tsx" --include="*.ts" -n
```

### 🔍 State Management Verification
```bash
# Check state update logic
grep -r "useState\|useEffect.*navigate" frontend/src --include="*.tsx" --include="*.ts" -n

# Check onClick handlers
grep -r "onClick.*=>" frontend/src --include="*.tsx" --include="*.ts" -n
```

### 🚨 Mandatory Verification Questions
For each user interaction, the reviewer must answer:

1. **What Happens on Click?**
   - What specific operations does the onClick handler perform?
   - Which functions are called? What parameters are passed?

2. **Where Does Navigation Go?**
   - What is the target path of navigate()?
   - Does this path exist in route configuration?
   - Is the path parameter format correct?

3. **What Does the Target Page Do?**
   - How does the target page/component handle URL parameters?
   - Are parameters correctly extracted and used?
   - What content does the user ultimately see?

4. **Is State Consistent?**
   - How does application state change after click?
   - Does UI correctly reflect state changes?
   - Are there any state synchronization issues?

**If the reviewer cannot answer these questions, the review must be marked as ❌ Fail**

## Quick Verification Tips
- Centralized Endpoint Source: Frontend prohibits hardcoded URLs; new/changed endpoints synchronized to constants/SDK.
- Authentication Consistency: Cross-origin/cross-port requests carry credentials (Cookie/Token) as needed, don't rely on undeclared custom headers.
- Async Fallback: Has fallback path and user prompts when events/streaming not supported or network errors occur.
- Visibility Scan: Critical entries visible in default state and target devices; empty/error/loading states reproducible and recoverable.
- Automated Checks: Add simple scripts/CI rules to check hardcoded endpoints, path format, required auth headers/credentials usage consistency.
