const apiUrl = "http://127.0.0.1:3000";


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



async function sendToBackend() {
    const searchValue = document.querySelector("#search-box").value;
    const regionValue = document.querySelector("select").value;
    const url = `${apiUrl}/search`;

    const riotUrl = `/lol/summoner/v4/summoners/by-name/${searchValue}`;

    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ url: riotUrl, region: regionValue })
    });

    if (response.ok) {
        const data = await response.json();

        // const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        // const link = document.createElement('a');
        // link.href = window.URL.createObjectURL(blob);
        // link.download = 'response.json';
        // document.getElementById('body').appendChild(link);
        // link.click();
        // document.getElementById('body').removeChild(link);
        
        document.querySelector("#profile-info").innerHTML = '';
        
        const allParticipants = data.data;
        
        // Group participants by matchId
        const matches = groupBy(allParticipants, 'matchId');
        console.log('Grouped matches:', matches); // Add this line

        const mainParticipant = allParticipants.find(participant => participant.isMainParticipant);
        if (mainParticipant) {
            const summonerSectionHTML = generateSummonerSection(mainParticipant);
            document.querySelector("#profile-info").innerHTML = summonerSectionHTML;
        }

        // For each match, find the main participant and the other participants
        Object.values(matches).forEach(matchParticipants => {
            const mainParticipant = matchParticipants.find(participant => participant.isMainParticipant);

            if (mainParticipant) {
                const participantCard = document.createElement('div');
                participantCard.className = `card ${mainParticipant.win ? 'win' : 'lose'}`;
                
                const kdaHTML = `<span class="kills">${mainParticipant.kills}</span>/<span class="deaths">${mainParticipant.deaths}</span>/<span class="assists">${mainParticipant.assists}</span>`;
                const statusText = mainParticipant.win ? 'Victory' : 'Defeat';
                
                const championImgUrl = `./league_data/img/champion/${mainParticipant.championName}.png`;
                const itemsGrid = generateItemsGrid(mainParticipant);

                participantCard.innerHTML = `
                    <img src="${championImgUrl}" alt="${mainParticipant.championName} Icon">
                    <div class="card-content">
                        <div class="kda-status-container">
                            <div class="card-title">${kdaHTML}</div>
                            <div class="card-status">${statusText}</div>
                        </div>
                        <div class="card-items">${itemsGrid}</div>
                        <div class="other-participants">
                            ${generateOtherParticipants(matchParticipants)}
                        </div>
                    </div>
                `;

                document.querySelector("#profile-info").appendChild(participantCard);
            }
        });
    } else {
        const errorText = await response.text();
        console.error("Failed to fetch data:", errorText);
        document.querySelector("#profile-info").innerHTML = `<div class="error">${errorText}</div>`;
    }
}


// Helper function to group an array of objects by a key
function groupBy(array, key) {
    return array.reduce((result, currentItem) => {
        (result[currentItem[key]] = result[currentItem[key]] || []).push(currentItem);
        return result;
    }, {});
}

function generateOtherParticipants(participants) {
    const leftParticipants = participants.slice(0, 5);
    const rightParticipants = participants.slice(5);

    const leftParticipantsHTML = leftParticipants.map(participant => {
        const championImgUrl = `./league_data/img/champion/${participant.championName}.png`;
        return `<div class="other-participant">
            <div class="other-participant-name">${participant.summonerName}</div>
            <img src="${championImgUrl}" alt="${participant.championName} Icon">
        </div>`;
    }).join('');

    const rightParticipantsHTML = rightParticipants.map(participant => {
        const championImgUrl = `./league_data/img/champion/${participant.championName}.png`;
        return `<div class="other-participant">
            <img src="${championImgUrl}" alt="${participant.championName} Icon">
            <div class="other-participant-name">${participant.summonerName}</div>
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






















document.getElementById("search-button").addEventListener("click", sendToBackend);
