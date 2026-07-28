document.querySelector("form").addEventListener("submit", async (e) => {
    e.preventDefault();

    const form = e.target;
    const errorEl = document.getElementById("login-error");
    errorEl.style.display = "none";

    const body = new URLSearchParams(new FormData(form));

    try {
        const res = await fetch("http://api.reggieguessr.com/login", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: body.toString(),
            credentials: "include",
        });

        const data = await res.json();

        if (res.ok) {
            window.location.href = "/home";
        } else {
            errorEl.textContent = data.error || "Login failed.";
            errorEl.style.display = "block";
        }
    } catch {
        errorEl.textContent = "Could not reach the server";
        errorEl.style.display = "block";
    }
});
