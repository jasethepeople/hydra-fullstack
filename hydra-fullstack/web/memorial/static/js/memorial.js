// Memorial Site Interactivity - Canvas, Chat, Uploads
let ws = null;
let selectedColor = '#ff6b6b';
let canvasPixels = new Map();

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    initCanvas();
    initWebSocket();
    initDragDrop();
    loadEntries();
});

// Collaborative Canvas
function initCanvas() {
    const canvas = document.getElementById('memorial-canvas');
    const ctx = canvas.getContext('2d');

    // Load existing pixels
    fetch('/memorial/canvas/pixels')
        .then(r => r.json())
        .then(pixels => {
            pixels.forEach(p => {
                ctx.fillStyle = p.color;
                ctx.fillRect(p.x, p.y, 4, 4);
                canvasPixels.set(`${p.x},${p.y}`, p.color);
            });
        })
        .catch(() => console.log('Canvas pixels not available'));

    canvas.addEventListener('click', (e) => {
        const rect = canvas.getBoundingClientRect();
        const x = Math.floor((e.clientX - rect.left) * (canvas.width / rect.width));
        const y = Math.floor((e.clientY - rect.top) * (canvas.height / rect.height));

        ctx.fillStyle = selectedColor;
        ctx.fillRect(x, y, 4, 4);

        fetch('/memorial/canvas', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ x, y, color: selectedColor, author: 'anonymous' })
        }).catch(() => {});

        canvasPixels.set(`${x},${y}`, selectedColor);
    });
}

function selectColor(btn) {
    document.querySelectorAll('.color-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    selectedColor = btn.dataset.color;
}

// WebSocket Chat
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/memorial/ws`);

    ws.onopen = () => console.log('WebSocket connected');

    ws.onmessage = (event) => {
        const msg = JSON.parse(event.data);
        if (msg.type === 'pixel') {
            const canvas = document.getElementById('memorial-canvas');
            const ctx = canvas.getContext('2d');
            ctx.fillStyle = msg.color;
            ctx.fillRect(msg.x, msg.y, 4, 4);
        } else if (msg.type === 'chat') {
            displayMessage(msg.author, msg.message, msg.timestamp);
        } else if (msg.type === 'welcome') {
            displayMessage('System', msg.message, new Date().toISOString());
        }
    };

    ws.onclose = () => setTimeout(initWebSocket, 3000);
}

function sendMessage() {
    const nameInput = document.getElementById('chat-name');
    const msgInput = document.getElementById('chat-message');
    const author = nameInput.value.trim() || 'Anonymous';
    const message = msgInput.value.trim();

    if (!message) return;

    const payload = {
        type: 'chat',
        author: author,
        message: message,
        timestamp: new Date().toISOString()
    };

    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(payload));
    }

    msgInput.value = '';
}

function displayMessage(author, message, timestamp) {
    const container = document.getElementById('chat-messages');
    const div = document.createElement('div');
    div.className = 'chat-message';
    const time = timestamp ? new Date(timestamp).toLocaleTimeString() : new Date().toLocaleTimeString();
    div.innerHTML = `<span class="author">${escapeHtml(author)}</span><span class="time">${time}</span><p>${escapeHtml(message)}</p>`;
    container.appendChild(div);
    container.scrollTop = container.scrollHeight;
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Drag & Drop Upload
function initDragDrop() {
    const dropZone = document.getElementById('drop-zone');
    if (!dropZone) return;

    dropZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropZone.classList.add('dragover');
    });

    dropZone.addEventListener('dragleave', () => {
        dropZone.classList.remove('dragover');
    });

    dropZone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropZone.classList.remove('dragover');
        const files = e.dataTransfer.files;
        if (files.length) handleFile(files[0]);
    });
}

function handleFileSelect(event) {
    const file = event.target.files[0];
    if (file) handleFile(file);
}

function handleFile(file) {
    const formData = new FormData();
    formData.append('media', file);
    formData.append('author_name', document.getElementById('chat-name').value || 'Anonymous');
    formData.append('content', document.getElementById('memory-text').value);

    fetch('/memorial/upload', { method: 'POST', body: formData })
        .then(r => r.json())
        .then(data => {
            alert('Memory shared successfully!');
            loadEntries();
        })
        .catch(err => alert('Upload failed: ' + err));
}

function submitMemory() {
    const text = document.getElementById('memory-text').value.trim();
    if (!text) return;

    fetch('/memorial/entries', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            entry_type: 'text',
            author_name: document.getElementById('chat-name').value || 'Anonymous',
            content: text,
            season_tag: getCurrentSeason()
        })
    })
    .then(() => {
        document.getElementById('memory-text').value = '';
        loadEntries();
    });
}

function loadEntries() {
    fetch('/memorial/entries?limit=12')
        .then(r => r.json())
        .then(data => {
            const grid = document.getElementById('entries-grid');
            if (!grid) return;
            grid.innerHTML = data.entries.map(e => `
                <div class="entry-card">
                    ${e.media_url ? `<img src="${e.media_url}" alt="Memory" loading="lazy">` : ''}
                    <div class="entry-content">
                        <div class="entry-author">${escapeHtml(e.author || 'Anonymous')}</div>
                        <div class="entry-text">${escapeHtml(e.content || '')}</div>
                        ${e.season ? `<span class="entry-season">${e.season}</span>` : ''}
                    </div>
                </div>
            `).join('');
        })
        .catch(() => console.log('Entries not available'));
}

function getCurrentSeason() {
    const month = new Date().getMonth() + 1;
    if (month >= 3 && month <= 5) return '🌸 Spring';
    if (month >= 6 && month <= 8) return '☀️ Summer';
    if (month >= 9 && month <= 11) return '🍂 Fall';
    return '❄️ Winter';
}
