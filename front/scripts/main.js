// Tabs
const tabs = document.querySelectorAll('.tab');
const indicator = document.getElementById('tabIndicator');

function setIndicator(tab) {
    indicator.style.left = tab.offsetLeft + 'px';
    indicator.style.width = tab.offsetWidth + 'px';
}

const savedActiveTab = localStorage.getItem("active-tab");
if(savedActiveTab != null) {
    document.getElementById(savedActiveTab+"-tab").classList.add("active");
    document.getElementById(savedActiveTab+"-panel").classList.add("active");
} else {
    document.getElementById("upload-tab").classList.add("active");
    document.getElementById("upload-panel").classList.add("active");
}

setIndicator(document.querySelector('.tab.active'));

tabs.forEach(tab => {
    tab.addEventListener('click', () => {
        document.querySelector('.tab.active').classList.remove('active');
        document.querySelector('.tab-panel.active').classList.remove('active');
        tab.classList.add('active');
        document.getElementById(tab.id.split("-")[0] + "-panel").classList.add('active');
        setIndicator(tab);
        localStorage.setItem("active-tab", tab.id.split("-")[0])
    });
});

// Download
const downloadList = document.getElementById('downloadList');

async function loadFiles() {
    downloadList.innerHTML = '';
    const res = await fetch('/api/files');
    const files = await res.json();

    if (files.length === 0) {
        downloadList.innerHTML = `
                    <div class="empty-state">
                        <p>No files uploaded yet.</p>
                    </div>`;
        return;
    }

    for (const file of files) {
        const item = document.createElement('div');
        item.className = 'download-item';
        const date = new Date(file.modified * 1000).toLocaleDateString('en-US', {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        });
        item.innerHTML = `
                    <div class="file-info">
                        <div class="file-name">${file.name}</div>
                        <div class="file-size">${formatBytes(file.size)}</div>
                        <div class="file-time">${date}</div>
                    </div>
                    <a href="/files/${file.id}" download="${file.name}" class="download-btn">Download</a>`;
        downloadList.appendChild(item);
    }
}

function formatBytes(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

document.getElementById("download-tab").addEventListener('click', loadFiles);
if(document.getElementById("download-tab").classList.contains("active")) {
    loadFiles();
}

// Upload
const uploadArea = document.getElementById('uploadArea');
const fileInput = document.getElementById('fileInput');
const fileList = document.getElementById('fileList');
const baseUrl = '/files/';

uploadArea.addEventListener('click', () => fileInput.click());

fileInput.addEventListener('change', (e) => {
    handleFiles(e.target.files);
});

function handleFiles(files) {
    for (let file of files) {
        uploadFile(file);
    }
}

function uploadFile(file) {
    const fileId = Math.random().toString(36);
    const fileItem = document.createElement('div');
    fileItem.className = 'file-item';
    fileItem.id = `file-${fileId}`;
    fileItem.innerHTML = `
                <div class="file-info">
                    <div class="file-name">${file.name}</div>
                    <div class="progress-bar">
                        <div class="progress-fill"></div>
                    </div>
                    <div class="file-status">Uploading...</div>
                </div>
            `;
    fileList.appendChild(fileItem);

    const upload = new tus.Upload(file, {
        endpoint: baseUrl,
        retryDelays: [0, 1000, 3000, 5000],
        metadata: {
            filename: file.name,
            filetype: file.type
        },
        onError(error) {
            console.log('Failed because: ' + error);
            updateFileStatus(fileId, 'Failed', 'error');
        },
        onProgress(bytesUploaded, bytesTotal) {
            const progress = (bytesUploaded / bytesTotal) * 100;
            updateProgressBar(fileId, progress);
        },
        onSuccess() {
            updateFileStatus(fileId, 'Complete', 'success');
        }
    });

    upload.start();
}

function updateProgressBar(fileId, progress) {
    const fileItem = document.getElementById(`file-${fileId}`);
    if (fileItem) {
        fileItem.querySelector('.progress-fill').style.width = progress + '%';
    }
}

function updateFileStatus(fileId, status, className) {
    const fileItem = document.getElementById(`file-${fileId}`);
    if (fileItem) {
        const statusEl = fileItem.querySelector('.file-status');
        statusEl.textContent = status;
        statusEl.className = `file-status ${className}`;
    }
}