document.addEventListener('DOMContentLoaded', () => {
    const recordButton = document.getElementById('record');
    const pauseButton = document.getElementById('pause');
    const stopButton = document.getElementById('stop');

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
});
