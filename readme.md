# Multi-Game Smart Contract (Tic-Tac-Toe, Connect Four, Gomoku)

This contract allows players to create, join, and play **Tic-Tac-Toe**, **Connect Four**, and **Gomoku** games on-chain. Bets can optionally be attached, and the contract handles moves, wins, draws, and payouts automatically.

---

## Game Types

| Game Type    | ID | Board Size | Winning Line Length |
| ------------ | -- | ---------- | ------------------- |
| Tic-Tac-Toe  | 1  | 3×3        | 3                   |
| Connect Four | 2  | 6×7        | 4                   |
| Gomoku       | 3  | 15×15      | 5                   |

---

## Quick Start

### 1. Create a Game

Call `createGame` with a **name** and **game type**. Optionally attach a bet via transaction intents.

**Example Payload:**

```json
{
  "name": "Fun Match",
  "type": 1
}
```

**Returns:** `nil`. Event `GameCreated` is emitted with the new game ID.

---

### 2. Join a Game

Call `joinGame(gameId)` to join a waiting game.

* Creator cannot join their own game.
* Bets must match exactly if one exists.

---

### 3. Make a Move

Call `makeMove(gameId, row, col)` to place your mark.

* **Tic-Tac-Toe / Gomoku:** specify row & column.
* **Connect Four:** specify column; the disc drops to the lowest empty row.

**Rules enforced:**

* Only the current player can make a move.
* Cell must be empty (or column must have space for Connect Four).

---

### 4. Game Completion

* Contract automatically checks for a win or draw after each move.
* Payouts occur automatically:

  * Winner receives the pot.
  * Draw splits the pot equally.

---

### 5. Timeout & Resign

* **Timeout:** If a player doesn’t move for 7 days, the opponent can claim a win by calling `claimTimeout(gameId)`.
* **Resign:** A player can forfeit by calling `resign(gameId)`; the other player wins automatically.

---

### 6. Check Game State

Call `getGame(gameId)` to retrieve the full game state:

**Example Response:**

```json
{
  "id": 1,
  "type": 1,
  "typeName": "TicTacToe",
  "name": "Fun Match",
  "creator": "addr1",
  "opponent": "addr2",
  "board": [0,1,0,2,0,0,0,0,0],
  "rows": 3,
  "cols": 3,
  "turn": 1,
  "moves_count": 3,
  "status": 1,
  "winner": null,
  "gameAsset": null,
  "gameBetAmount": null,
  "lastMoveAt": "2025-09-26T20:45:00Z"
}
```

* `board` stores the game cells as 2-bit packed values (0 = empty, 1 = X, 2 = O).
* `turn` indicates the next player (`1 = X`, `2 = O`).
* `status` = 0 (waiting), 1 (in progress), 2 (finished).

---

### 7. Bets

* Supported via `sdk.Asset` and amount in `GameBetAmount`.
* Automatically handled for winner or split in case of draw.

---

### 8. Events

* **GameCreated(gameId, creator)**
* **GameJoined(gameId, joiner)**
* **GameMoveMade(gameId, player, position)**
* **GameWon(gameId, winner)**
* **GameDraw(gameId)**
* **GameResigned(gameId, player)**

These events can be used to track game progress off-chain.

---

### Notes

* Each game board is stored **efficiently** as a `[]byte` with 2 bits per cell.
* Maximum board size: 15×15 for Gomoku.
* Supports multiple games simultaneously.
