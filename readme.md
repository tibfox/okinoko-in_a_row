# 🎮 Ōkinoko In-A-Row: On-Chain Turn-Based Game Engine (TicTacToe, ConnectFour, Gomoku)

This contract implements a fully on-chain, turn-based board game engine that supports **Tic-Tac-Toe**, **Connect Four**, and **Gomoku**, including optional **betting using token transfer intents** on the [vsc ecosystem](https://github.com/vsc-eco/).

Players interact via WASM-exported functions, sending human-readable strings as payloads. All game state is maintained on-chain, with turn enforcement, timeouts, betting, victory detection, resignation logic, and querying functions. Game workflow and lobby will soon be available on the [Ōkinoko Terminal](https://terminal.okinoko.io/).

---

## 📦 **Supported Game Types**

| Value | Game        | Board Size | Win Condition             |
| ----- | ----------- | ---------- | ------------------------- |
| 1     | [TicTacToe](https://en.wikipedia.org/wiki/Tic-tac-toe)   | 3x3        | 3 in a row                |
| 2     | [ConnectFour](https://en.wikipedia.org/wiki/Connect_Four) | 6x7        | 4 in a row (gravity drop) |
| 3     | [Gomoku](https://en.wikipedia.org/wiki/Gomoku)      | 15x15      | 5 in a row                |

---

# 🚀 Exported Entry Points (Public API)

Each exported function accepts a string, where fields are separated by `|`. Inputs and outputs are plain UTF-8 text.

---

## 👉 1. **Create Game**

**Export name:** `g_create`
**Signature:** `func CreateGame(payload *string) *string`

### **Input Format**

```
"type|name"
```

* `type`: `1` | `2` | `3`
* `name`: Human-readable string, must not contain `|`

### **Optional Betting**

To place a bet, the caller must provide a `transfer.allow` intent with selected asset (`hive` or `hbd`) and a limit.

### **Returns**

* `<gameId>` on success

---

## 👉 2. **Join Game**

**Export name:** `g_join`
**Signature:** `func JoinGame(payload *string) *string`

### **Input Format**

```
"gameId"
```

If the game has an existing bet, the joining player must match it with a transfer intent.

### **Returns**

* `nil` on success

---

## 👉 3. **Make Move**

**Export name:** `g_move`
**Signature:** `func MakeMove(payload *string) *string`

### **Input Format**

```
"gameId|row|col"
```

For Connect Four, `row` is ignored by the caller; the contract computes it.

### **Game Rules Enforced**

* Must be the caller’s turn
* Position must be valid and unoccupied
* Automatically checks win or draw
* Transfers pot to winner (if betting enabled)

### **Returns**

* `nil` on success

---

## 👉 4. **Claim Timeout**

**Export name:** `g_timeout`
**Signature:** `func ClaimTimeout(payload *string) *string`

### **Input Format**

```
"gameId"
```

If the opposing player has not made a move within 7 days, the caller may claim a win.

### **Rules**

* Only the waiting player may call
* Betting pot is transferred to winner

### **Returns**

* `nil` on success

---

## 👉 5. **Resign**

**Export name:** `g_resign`
**Signature:** `func Resign(payload *string) *string`

### **Input Format**

```
"gameId"
```

A player may resign at any time:

* If no opponent has joined, the game is simply removed
* If opponent exists, the opponent automatically wins

### **Returns**

* `nil` on success

---

## 🔍 Query Functions

### **6. Get Game State**

**Export name:** `g_get`
**Signature:** `func GetGame(payload *string) *string`

#### Input:

```
"gameId"
```

#### Output Format:

```
"id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|BoardContent"
```

* `BoardContent`: ASCII digits representing each cell (`0=empty`, `1=X`, `2=O`)
* Rows are appended row-by-row, no separators

---

### **7. List Waiting Games**

**Export name:** `g_waiting`
**Signature:** `func GetWaitingGames(_ *string) *string`

#### Output:

```
"1,2,3"
```

Comma-separated list of game IDs awaiting opponents.

---

# 🔐 Timeout Rules

* Timeout period: **7 days**
* Calculated from `LastMoveAt`
* Only the player *waiting for the opponent’s move* can claim a timeout

---

# 💸 Betting via Transfer Intents

| Field | Meaning                 |
| ----- | ----------------------- |
| token | Must be `hive` or `hbd` |
| limit | Decimal amount (float)  |

* First player’s bet is deducted on game creation.
* Second player must match bet when joining.
* Winner receives entire pot.
* In a draw, pot is split evenly.

---

# ⚙ Status Codes

| Status Value | Meaning            |
| ------------ | ------------------ |
| 0            | Waiting for player |
| 1            | In progress        |
| 2            | Finished           |

| Turn Value | Player       |
| ---------- | ------------ |
| 1          | X (creator)  |
| 2          | O (opponent) |

---

# 📄 Example API Usage

### 🎯 Create TicTacToe Game

Input:

```
"1|My First Game"
```

Output:

```
"5"
```

### 🎯 Join Game with ID 5

Input:

```
"5"
```

### 🎯 Make Move to Row 1, Col 2

Input:
```
"5|1|2"
```


### 🎯 Retrieve Game State

Input:

```
"5"
```

Output:

```
"5|1|My First Game|hive:alice|hive:bob|3|3|2|5|2|hive:alice|hive|1000|1720000000|111020200"

```
for this game board: 
```
X | X | X
O |   |  
O |   |  
```


## 📜 License

This project is licensed under the [MIT License](LICENSE).