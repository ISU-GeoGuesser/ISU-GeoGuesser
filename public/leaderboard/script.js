async function loadLeaderboard() {
  let data;
  try {
    const response = await fetch("https://api.reggieguessr.com/leaderboard?number_of_players=5");
    if (!response.ok) throw new Error(`status ${response.status}`);
    data = await response.json();
  } catch (err) {
    console.error("Failed to load leaderboard:", err);
    return;
  }

  const table = document.querySelector("table");

  for (const player of data) {
    const row = document.createElement("tr");

    const nameCell = document.createElement("td");
    nameCell.textContent = player.username;

    const scoreCell = document.createElement("td");
    scoreCell.textContent = player.score;

    row.appendChild(nameCell);
    row.appendChild(scoreCell);
    table.appendChild(row);
  }
}

loadLeaderboard();