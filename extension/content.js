let recording = false;

// Inject the styles into the page
function injectStyles() {
    const style = document.createElement('style');
    style.innerHTML = `
        #toolbar button {
            transition: background-color 0.3s ease;
        }

        #toolbar button:hover {
            background-color: #0056b3;  /* Darker blue on hover */
        }

        #toolbar button:disabled:hover {
            background-color: #6c757d !important;  /* Disabled button remains the same */
            cursor: not-allowed;
        }

        #toolbar button#stop:hover {
            background-color: #c82333;  /* Darker red for the stop button on hover */
        }
    `;
    document.head.appendChild(style);
}

// Inject the toolbar into the page
function createToolbar() {
    const toolbar = document.createElement('div');
    toolbar.id = 'toolbar';
    toolbar.style.position = 'fixed';
    toolbar.style.top = '10px';
    toolbar.style.right = '10px';
    toolbar.style.background = '#f1f1f1';
    toolbar.style.border = '1px solid #ccc';
    toolbar.style.padding = '15px';
    toolbar.style.borderRadius = '8px';
    toolbar.style.boxShadow = '0px 0px 10px rgba(0, 0, 0, 0.1)';
    toolbar.style.zIndex = '9999';
    toolbar.style.display = 'flex';
    toolbar.style.flexDirection = 'column';
    toolbar.style.alignItems = 'center';
    toolbar.style.width = '200px';

    // Title
    const title = document.createElement('h3');
    title.innerText = 'Recorder';
    title.style.margin = '0 0 10px 0';
    title.style.fontSize = '18px';
    title.style.color = '#333';
    toolbar.appendChild(title);

    // Add buttons
    const buttonStyle = `
        width: 100%;
        padding: 10px;
        margin: 5px 0;
        font-size: 14px;
        border-radius: 5px;
        border: none;
        cursor: pointer;
        background-color: #007BFF;
        color: white;
    `;

    const recordButton = document.createElement('button');
    recordButton.innerText = 'Start Recording';
    recordButton.style.cssText = buttonStyle;

    const pauseButton = document.createElement('button');
    pauseButton.innerText = 'Pause Recording';
    pauseButton.disabled = true;
    pauseButton.style.cssText = buttonStyle;
    pauseButton.style.backgroundColor = '#6c757d';

    const stopButton = document.createElement('button');
    stopButton.innerText = 'Stop Recording';
    stopButton.disabled = true;
    stopButton.style.cssText = buttonStyle;
    stopButton.style.backgroundColor = '#DC3545';

    // Append buttons to toolbar
    toolbar.appendChild(recordButton);
    toolbar.appendChild(pauseButton);
    toolbar.appendChild(stopButton);

    // Append toolbar to document
    document.body.appendChild(toolbar);

    // Button event listeners
    recordButton.addEventListener('click', () => {
        browser.runtime.sendMessage({ action: 'start_recording' });
        recordButton.disabled = true;
        pauseButton.disabled = false;
        stopButton.disabled = false;
    });

    pauseButton.addEventListener('click', () => {
        browser.runtime.sendMessage({ action: 'pause_recording' });
        pauseButton.disabled = true;
        recordButton.disabled = false;
    });

    stopButton.addEventListener('click', () => {
        browser.runtime.sendMessage({ action: 'stop_recording' });
        recordButton.disabled = false;
        pauseButton.disabled = true;
        stopButton.disabled = true;
    });
}

// Inject styles and toolbar
injectStyles();
createToolbar();

// Automatically start/stop recording based on background script messages
browser.runtime.onMessage.addListener((message) => {
    if (message.action === 'start_recording') {
        recording = true;
    }
    if (message.action === 'pause_recording') {
        recording = false;
    }
    if (message.action === 'stop_recording') {
        recording = false;
    }
});

// Send recorded interaction to the background script
function sendInteraction(interaction) {
    browser.runtime.sendMessage({ interaction });
}

// Format the interaction in the Go-compatible format
function formatInteraction(type, selector, text = null, value = null, keys = [], params = {}) {
    return {
        action: type,
        selector: selector,
        text: text,
        value: value,
        keys: keys.length ? keys : undefined,
        params: Object.keys(params).length ? params : undefined,
        timestamp: Date.now()
    };
}

// Capture clicks (used for clicks and dropdown selection)
function handleClick(event) {
    if (!recording) return;
    const selector = getUniqueSelector(event.target);

    // If the target is a select dropdown, capture the selected value
    if (event.target.tagName.toLowerCase() === 'select') {
        sendInteraction(formatInteraction('click', selector, null, event.target.value));
    } else {
        sendInteraction(formatInteraction('click', selector));
    }
}

// Capture typing in input fields (for text inputs)
function handleInput(event) {
    if (!recording) return;
    const selector = getUniqueSelector(event.target);
    if (event.target.tagName.toLowerCase() === 'input' || event.target.tagName.toLowerCase() === 'textarea') {
        sendInteraction(formatInteraction('enter_text', selector, event.target.value));
    }
}

// Capture keypresses
function handleKeyPress(event) {
    if (!recording) return;
    const selector = getUniqueSelector(event.target);
    sendInteraction(formatInteraction('keypress', selector, null, null, [event.key]));
}

// Get a unique selector for the clicked element
function getUniqueSelector(element) {
    if (element.id) {
        return `#${element.id}`;
    }
    if (element.className) {
        return `${element.tagName.toLowerCase()}.${element.className.split(' ').join('.')}`;
    }
    return element.tagName.toLowerCase();
}

// Add event listeners for various interactions
document.addEventListener('click', handleClick, true);
document.addEventListener('input', handleInput, true);
document.addEventListener('keypress', handleKeyPress, true);
