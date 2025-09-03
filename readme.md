# Tic-Tac-Toe VSC Smart Contract 🎮

*A game contract for the [vsc-eco](https://github.com/vsc-eco) ecosystem*

This contract brings **Tic-Tac-Toe** (a.k.a. “X and O”) onto the VSC blockchain.
It allows two players to start a game, make moves on-chain, wager assets, and claim winnings automatically.

---

## 🔑 Key Features

* **Create Games** — anyone can start a new Tic-Tac-Toe match with a unique game ID.
* **Join Games** — other players can join open matches.
* **On-Chain Gameplay** — moves are validated and stored on the blockchain.
* **Resign** — players can resign at any point, awarding the win to the opponent.
* **Automatic Payouts** — when bets are included, the contract distributes winnings to the winner (or splits in case of a draw).
* **Game Queries** — fetch games by ID, creator, player, or status (waiting, in progress, finished).

---

## 🕹️ How to Play

### 1. Create a Game

Call the contract’s `createGame` function with a chosen `gameId`.
Optionally, attach a bet by including a transfer intent.

Example payload:

```json
{
  "gameId": "game123"
}
```

If a bet is included, the contract locks the funds until the game is resolved.

---

### 2. Join a Game

Another player joins with `joinGame`.

* If the game has a bet, the joiner must provide the same token and amount.
* The game then moves to **In Progress**.

---

### 3. Make Moves

Use `makeMove` to place your mark (`X` for the creator, `O` for the joiner).
Payload format:

```json
{
  "gameId": "game123",
  "pos": 4
}
```

* `pos` is a number from 0–8 representing the board (left to right, top to bottom).
* The contract checks turn order, valid moves, and winning conditions.

---

### 4. Resign

At any time, a player can resign with `resign`.

* If the creator resigns → the opponent wins.
* If the opponent resigns → the creator wins.
* Locked bets are transferred accordingly.

---

### 5. Winning and Draws

* First player to get **3 in a row** wins.
* If all 9 cells are filled with no winner → the game ends in a **draw**.
* Payout logic:

  * Winner takes all (double the bet).
  * In a draw, both players get their stake back.

---

## 📊 Querying Games

You can fetch games in different ways:

* `getGame(gameId)` → get the full game state.
* `getGameForCreator(address)` → all games created by an address.
* `getGameForPlayer(address)` → all games played by an address.
* `getGameForGameState(state)` → list games by status:

  * `0 = WaitingForPlayer`
  * `1 = InProgress`
  * `2 = Finished`

---

## 🧩 Game State Model

Each game tracks:

* **ID** — unique string identifier.
* **Creator / Opponent** — player addresses.
* **Board** — 9-cell array (`X`, `O`, or empty).
* **Turn** — which player goes next.
* **Status** — waiting, in progress, or finished.
* **Winner** — set when the game ends.
* **GameAsset / GameBetAmount** — optional betting token and amount.

---

## Example Flow

1. Player A creates `game42` with a 100-token bet.
2. Player B joins `game42`, also staking 100 tokens.
3. Players alternate moves until Player A wins.
4. Contract transfers 200 tokens to Player A automatically.
5. Game status changes to **Finished**.

---

⚡ Have fun playing Tic-Tac-Toe on-chain, with fairness and trust guaranteed by the contract!

