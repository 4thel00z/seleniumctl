let isRecording = false;
let interactions = [];

browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.action === 'start_recording') {
        isRecording = true;
        interactions = [];

        // Get the current active tab and record its URL as the first interaction
        browser.tabs.query({ active: true, currentWindow: true }).then((tabs) => {
            const currentTab = tabs[0];
            interactions.push({
                action: 'navigate',
                url: currentTab.url,
                timestamp: Date.now()
            });

            // Notify content script to start recording user interactions
            browser.tabs.sendMessage(tabs[0].id, { action: 'start_recording' });
        });
    }

    if (message.action === 'pause_recording') {
        isRecording = false;
        browser.tabs.query({ active: true, currentWindow: true }).then((tabs) => {
            browser.tabs.sendMessage(tabs[0].id, { action: 'pause_recording' });
        });
    }

    if (message.action === 'stop_recording') {
        isRecording = false;
        browser.tabs.query({ active: true, currentWindow: true }).then((tabs) => {
            browser.tabs.sendMessage(tabs[0].id, { action: 'stop_recording' });
        });
        saveInteractions();
    }

    if (message.interaction && isRecording) {
        interactions.push(message.interaction);
    }

    return Promise.resolve();
});

function saveInteractions() {
    const data = JSON.stringify(interactions, null, 2);
    const blob = new Blob([data], { type: 'application/json' });
    const url = URL.createObjectURL(blob);

    browser.downloads.download({
        url: url,
        filename: `interactions_${Date.now()}.json`,
        saveAs: false
    }).then(() => {
        console.log('Interactions saved successfully.');
    }).catch((error) => {
        console.error('Error saving interactions:', error);
    });
}
