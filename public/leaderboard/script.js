async function loadLeaderboard() {
  const response = await fetch("https://reggieguessr.com/leaderboard?number_of_players=5");
  const data = await response.json();
  console.log(data); 

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