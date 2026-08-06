document.querySelector("form").addEventListener("submit", async (e) => {
    e.preventDefault();

    const form = e.target;
    const errorEl = document.getElementById("register-error");
    errorEl.style.display = "none";

    const body = new URLSearchParams(new FormData(form));

    try {
        const res = await fetch("https://api.reggieguessr.com/register", {
            method: "POST",
            headers: { "Content-Type": "application/x-www-form-urlencoded" },
            body: body.toString(),
            credentials: "include",
        });

        const data = await res.json();

        if (res.ok) {
            window.location.href = "/auth/login";
        } else {
            errorEl.textContent = data.error || "Registration failed.";
            errorEl.style.display = "block";
        }
    } catch {
        errorEl.textContent = "Could not reach the server. Please try again.";
        errorEl.style.display = "block";
    }
});
