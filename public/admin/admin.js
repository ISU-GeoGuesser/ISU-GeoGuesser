const input = document.getElementById('image-uploader');
const fileList = document.getElementById('file-list');
const uploadBtn = document.getElementById('upload-btn');
const statusEl = document.getElementById('status');

input.addEventListener('change', () => {
    fileList.innerHTML = '';
    if (!input.files.length) {
        uploadBtn.disabled = true;
        return;
    }
    Array.from(input.files).forEach((file, i) => {
        const entry = document.createElement('div');
        entry.className = 'file-entry';

        const lbl = document.createElement('label');
        lbl.textContent = file.name;
        lbl.title = file.name;

        const nameInput = document.createElement('input');
        nameInput.type = 'text';
        nameInput.placeholder = 'Location name';
        nameInput.dataset.index = i;
        nameInput.required = true;

        entry.appendChild(lbl);
        entry.appendChild(nameInput);
        fileList.appendChild(entry);
    });
    uploadBtn.disabled = false;
});

uploadBtn.addEventListener('click', async () => {
    const files = input.files;
    const nameInputs = fileList.querySelectorAll('input[type="text"]');
    const results = [];

    uploadBtn.disabled = true;
    statusEl.textContent = '';

    for (let i = 0; i < files.length; i++) {
        const name = nameInputs[i].value.trim();
        if (!name) {
            results.push(`[SKIP] ${files[i].name} — name is required`);
            continue;
        }

        const formData = new FormData();
        formData.append('name', name);
        formData.append('image', files[i]);

        try {
            const res = await fetch('https://api.reggieguessr.com/locations', {
                method: 'POST',
                body: formData,
                credentials: 'include',
            });
            const json = await res.json();
            if (res.ok) {
                results.push(`[OK]   ${files[i].name} → ${name}`);
            } else {
                results.push(`[ERR]  ${files[i].name} - ${json.error || res.statusText}`);
            }
        } catch (err) {
            results.push(`[ERR]  ${files[i].name} — ${err.message}`);
        }

        statusEl.textContent = results.join('\n');
    }

    uploadBtn.disabled = false;
});