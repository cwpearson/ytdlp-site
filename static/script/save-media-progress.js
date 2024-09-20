
// Get all video and audio elements
const mediaElements = document.querySelectorAll('video, audio');

// Generate a unique key for this page
const pageKey = `mediaProgress_${window.location.pathname}`;

// Function to save the current time of the most recently played media
function saveMediaProgress(media) {
    const currentProgress = parseFloat(localStorage.getItem(pageKey)) || 0;
    if (media.currentTime > currentProgress) {
        localStorage.setItem(pageKey, media.currentTime);
    }
}

// Function to load and set the saved time for all media elements
function loadMediaProgress() {
    const savedTime = localStorage.getItem(pageKey);
    if (savedTime) {
        mediaElements.forEach(media => {
            media.currentTime = Math.min(parseFloat(savedTime), media.duration || Infinity);
        });
    }
}

// Set up event listeners for each media element
mediaElements.forEach(media => {
    // Save progress when the media is playing
    media.addEventListener('timeupdate', () => {
        if (!media.paused) {
            saveMediaProgress(media);
        }
    });

    // Also save when the media is paused
    media.addEventListener('pause', () => saveMediaProgress(media));

    // Load the saved progress when the media is ready
    media.addEventListener('loadedmetadata', loadMediaProgress);

    // Clear progress when any media ends
    media.addEventListener('ended', () => {
        localStorage.removeItem(pageKey);
    });
});