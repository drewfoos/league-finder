const apiUrl = "http://127.0.0.1:3000";
window.searchSummoner = searchSummoner;
import imagesLoaded from 'imagesloaded';

let isFreshSearch = true; // Track if it's a fresh search

document.getElementById("show-more").addEventListener("click", showMore);

let start = 0;
let count = 5;

async function showMore() {
    const showMoreButton = document.getElementById("show-more");
    showMoreButton.disabled = true;
    showMoreButton.textContent = "Loading...";
    
    start += count;
    await sendToBackend();
    
    showMoreButton.disabled = false;
    showMoreButton.textContent = "Show More";
}



function preloadImage(src) {
    return new Promise((resolve, reject) => {
        const img = new Image();
        img.src = src;
        img.onload = resolve;
        img.onerror = reject;
    });
}

function generateItemsGrid(participant) {
    const items = [participant.item0, participant.item1, participant.item2, participant.item3, participant.item4, participant.item5];
    const validItems = items.filter(item => item !== 0); // Filtering out items with ID 0

    return validItems.map(item => `<img src="./league_data/img/item/${item}.png" alt="Item ${item}">`).join('');
}

function generateSummonerSection(participant) {
    const profileIconUrl = `./league_data/img/profileicon/${participant.profileIcon}.png`;
    const tierAndRank = (participant.tier === 'Master' || participant.tier === 'Challenger') ? 
                        `${participant.tier} (${participant.leaguePoints} LP)` : 
                        `${participant.tier} ${participant.rank} (${participant.leaguePoints} LP)`;
    const winsLosses = `Wins: ${participant.wins} - Losses: ${participant.losses}`;

    const emblemOrText = participant.tier !== "" ?
        `<img src="./league_data/img/ranked-emblem/${participant.tier}.png" alt="${participant.tier} emblem" class="rank-emblem">` :
        'No ranked games';
    
    return `
        <div class="summoner-section">
            <div class="summoner-details-left">
                <img src="${profileIconUrl}" alt="${participant.summonerName}'s Profile Icon">
                <div class="summoner-name">${participant.summonerName}</div>
            </div>
            <div class="divider">|</div>
            <div class="summoner-details-right">
                ${emblemOrText}
                <div class="tier-rank">${tierAndRank}</div>
                <div class="wins-losses">${winsLosses}</div>
            </div>
        </div>
    `;
}



const preloadedImages = new Set();

async function sendToBackend() {

    document.getElementById("loading").style.display = 'block';

    if (isFreshSearch) {
        count = 5;
        start = 0;
        document.querySelector("#profile-info").innerHTML = ''; // Clear existing content only for fresh searches
        document.getElementById("show-more").style.display = 'none';
        isFreshSearch = false;
    }

    const searchValue = document.querySelector("#search-box").value;
    sessionStorage.setItem('lastSearched', searchValue);
    const regionValue = document.querySelector("select").value;
    const url = `${apiUrl}/search`;

    const riotUrl = `/lol/summoner/v4/summoners/by-name/${searchValue}`;

    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ url: riotUrl, region: regionValue, count: count, start: start, })
        });

        if (!response.ok) {
            const errorData = await response.json();
            const errorText = `Error: ${errorData.error}`;
            console.error(errorText);
            document.querySelector("#profile-info").innerHTML = `<div class="error">${errorText}</div>`;
            document.getElementById("loading").style.display = 'none';
            return;
        }

        const data = await response.json();
        const allParticipants = data.data;

        // Group participants by matchId
        const matches = groupBy(allParticipants, 'matchId');

        const mainParticipant = allParticipants.find(participant => participant.isMainParticipant);

        const fragment = document.createDocumentFragment();

        if (mainParticipant && start === 0) {
            const summonerSectionHTML = generateSummonerSection(mainParticipant);
            const summonerSection = document.createElement('div');
            summonerSection.innerHTML = summonerSectionHTML;
            fragment.appendChild(summonerSection);
        }

        const preloadPromises = [];

        // For each match, find the main participant and the other participants
        for (const matchParticipants of Object.values(matches)) {
            const mainParticipant = matchParticipants.find(participant => participant.isMainParticipant);

            if (mainParticipant) {
                const participantCard = document.createElement('div');
                participantCard.className = `card ${mainParticipant.win ? 'win' : 'lose'}`;

                const kdaHTML = `<span class="kills">${mainParticipant.kills}</span>/<span class="deaths">${mainParticipant.deaths}</span>/<span class="assists">${mainParticipant.assists}</span>`;
                const statusText = mainParticipant.win ? 'Victory' : 'Defeat';

                const championImgUrl = `./league_data/img/champion/${mainParticipant.championName}.png`;

                if (!preloadedImages.has(championImgUrl)) {
                    preloadPromises.push(preloadImage(championImgUrl));
                    preloadedImages.add(championImgUrl);
                }

                // Preload other participants' images
                matchParticipants.forEach(participant => {
                    const championImgUrl = `./league_data/img/champion/${participant.championName}.png`;

                    if (!preloadedImages.has(championImgUrl)) {
                        preloadPromises.push(preloadImage(championImgUrl));
                        preloadedImages.add(championImgUrl);
                    }
                });

                const itemsGrid = generateItemsGrid(mainParticipant);


                participantCard.innerHTML = `
                    <img src="${championImgUrl}" alt="${mainParticipant.championName} Icon">
                    <div class="card-content">
                        <div class="kda-status-container">
                            <div class="queue-description">${mainParticipant.queueDescription}</div>
                            <div class="card-title">${kdaHTML}</div>
                            <div class="card-status">${statusText}</div>
                        </div>
                        <div class="card-items">${itemsGrid}</div>
                        <div class="other-participants">
                            ${generateOtherParticipants(matchParticipants)}
                        </div>
                    </div>
                `;

                fragment.appendChild(participantCard);
            }
        }

        await Promise.all(preloadPromises);

        document.querySelector("#profile-info").appendChild(fragment);

        // Hide loading message when all images are loaded
        imagesLoaded(document.querySelector("#profile-info"), function() {
            document.getElementById("loading").style.display = 'none';
        });
        document.getElementById("show-more").style.display = 'block';
    } catch (error) {
        console.error('Error:', error);
        document.querySelector("#profile-info").innerHTML = `<div class="error">Error: ${error.message}</div>`;
        document.getElementById("loading").style.display = 'none';
        document.getElementById("show-more").style.display = 'block';
    }
}


document.addEventListener('DOMContentLoaded', (event) => {
    const lastSearched = sessionStorage.getItem('lastSearched');
    if (lastSearched) {
        document.querySelector("#search-box").value = lastSearched;
        sendToBackend();
    }
});


// Helper function to group an array of objects by a key
function groupBy(array, key) {
    return array.reduce((result, currentItem) => {
        (result[currentItem[key]] = result[currentItem[key]] || []).push(currentItem);
        return result;
    }, {});
}




function searchSummoner(event, summonerName) {
    event.stopPropagation();
    document.querySelector("#search-box").value = summonerName;
    isFreshSearch = true; // Mark as a fresh search
    sendToBackend();
}


document.getElementById("search-button").addEventListener("click", function() {
    isFreshSearch = true;
    sendToBackend();
});

function generateOtherParticipants(participants) {
    const leftParticipants = participants.slice(0, 5);
    const rightParticipants = participants.slice(5);

    const leftParticipantsHTML = leftParticipants.map(participant => {
        const championImgUrl = `./league_data/img/champion/${participant.championName}.png`;
        return `<div class="other-participant left-participant">
            <div class="other-participant-name" onclick="searchSummoner(event, '${participant.summonerName}')">${participant.summonerName}</div>
            <img src="${championImgUrl}" alt="${participant.championName} Icon">
        </div>`;
    }).join('');

    const rightParticipantsHTML = rightParticipants.map(participant => {
        const championImgUrl = `./league_data/img/champion/${participant.championName}.png`;
        return `<div class="other-participant right-participant">
            <img src="${championImgUrl}" alt="${participant.championName} Icon">
            <div class="other-participant-name" onclick="searchSummoner(event, '${participant.summonerName}')">${participant.summonerName}</div>
        </div>`;
    }).join('');

    return `<div class="participants-container">
        <div class="participants-column">
            ${leftParticipantsHTML}
        </div>
        <div class="participants-column">
            ${rightParticipantsHTML}
        </div>
    </div>`;
}