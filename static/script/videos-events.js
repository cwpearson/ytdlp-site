// Function to disable the auto-refresh
function disableAutoRefresh() {
    // Find the meta tag that controls the refresh
    var metaRefresh = document.querySelector('meta[http-equiv="refresh"]');

    // If the meta tag exists, remove it
    if (metaRefresh) {
        metaRefresh.remove();
        console.log("Auto-refresh disabled");
    } else {
        console.log("No auto-refresh meta tag found");
    }
}

const eventSource = new EventSource('/videos/events');

function closeEventSource() {
    if (eventSource) {
        eventSource.close();
    }
}

function hideDivs(parent, hide, classes) {
    classes.forEach(cls => {
        divs = parent.querySelectorAll('div' + cls)
        console.log(divs)
        if (hide) {
            divs.forEach(div => div.classList.add("hidden"))
        } else {
            divs.forEach(div => div.classList.remove("hidden"))
        }
    })
}

function showDivs(parent, show, classes) {
    classes.forEach(cls => {
        divs = parent.querySelectorAll(cls)
        console.log(divs)
        if (show) {
            divs.forEach(div => div.classList.remove("hidden"))
        } else {
            divs.forEach(div => div.classList.add("hidden"))
        }
    })
}

function updateCardsStyling(card) {
    console.log(`updateCardsStyling: card:`, card);

    const statusDiv = card.querySelector('.video-status');
    if (statusDiv) {
        const statusText = statusDiv.textContent.trim().toLowerCase();

        if (["completed", "download completed", "transcoding"].includes(statusText)) {
            hideDivs(card, false, [".video-title-link"])
            hideDivs(card, true, [".video-title-bare"])
        } else { // failed
            hideDivs(card, true, [".video-title-link"])
            hideDivs(card, false, [".video-title-bare"])
        }

        showDivs(card, (statusText == "completed"), [".reprocess-btn", ".delete-btn"])
        showDivs(card, (statusText == "failed"), [".restart-btn"])

    }
}

eventSource.onmessage = function (event) {
    const data = JSON.parse(event.data);
    console.log(data)

    const videoCard = document.getElementById(`video-card-${data.VideoId}`);
    if (videoCard) {
        const statusDiv = videoCard.querySelector('.video-info.video-status');
        if (statusDiv) {
            statusDiv.textContent = data.Status;
        } else {
            console.error(`Status div not found for video ID ${data.VideoId}`);
        }
    } else {
        console.error(`Video card not found for ID ${data.VideoId}`);
    }

    updateCardsStyling(videoCard)
};

eventSource.onopen = function (event) {
    console.log("Connection to server opened.");
    disableAutoRefresh();
};

eventSource.onerror = function (error) {
    console.error('EventSource failed:', error);
    eventSource.close();
};

// Add event listener for when the page is about to unload
window.addEventListener('beforeunload', closeEventSource);

// Call the function when the page loads
// window.onload = disableAutoRefresh;