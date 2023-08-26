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
    
    return `
        <div class="summoner-section">
            <div class="summoner-details-left">
                <img src="${profileIconUrl}" alt="${participant.summonerName}'s Profile Icon">
                <div class="summoner-name">${participant.summonerName}</div>
            </div>
            <div class="divider">|</div>
            <div class="summoner-details-right">
                <img src="./league_data/img/ranked-emblem/${participant.tier}.png" alt="${participant.tier} emblem" class="rank-emblem">
                <div class="tier-rank">${tierAndRank}</div>
                <div class="wins-losses">${winsLosses}</div>
            </div>
        </div>
    `;
}








async function sendToBackend() {
    const searchValue = document.querySelector("#search-box").value;
    const regionValue = document.querySelector("select").value; // Fetch the value of the dropdown
    const url = `${apiUrl}/search`;

    const riotUrl = `/lol/summoner/v4/summoners/by-name/${searchValue}`; // Remove the domain from the URL

    const response = await fetch(url, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ url: riotUrl, region: regionValue }) // Send the region as part of the request
    });

    if (response.ok) {
        const data = await response.json();
        
        // Clear previous results
        document.querySelector("#profile-info").innerHTML = '';

        // Display the summoner section once, using the data of the first participant as a sample
        if (data.data.length > 0) {
            const summonerSectionHTML = generateSummonerSection(data.data[0]);
            document.querySelector("#profile-info").innerHTML = summonerSectionHTML;
        }

        // Display each participant's data
        data.data.forEach(participant => {
            const participantDiv = document.createElement('div');
            const tierAndRank = `${participant.tier} ${participant.rank} (${participant.leaguePoints} LP)`;
            participantDiv.className = `card ${participant.win ? 'win' : 'lose'}`;

        
            const kdaHTML = `
                <span class="kills">${participant.kills}</span>/<span class="deaths">${participant.deaths}</span>/<span class="assists">${participant.assists}</span>
            `;
        
            const championImgUrl = `./league_data/img/champion/${participant.championName}.png`;
            const itemsGrid = generateItemsGrid(participant);
        
            const statusText = participant.win ? 'Victory' : 'Defeat';
            participantDiv.innerHTML = `
                <img src="${championImgUrl}" alt="${participant.championName} Icon">
                <div class="card-content">
                    <div class="kda-status-container">
                        <div class="card-title">${kdaHTML}</div>
                        <div class="card-status">${statusText}</div>
                    </div>
                    <div class="card-items">${itemsGrid}</div>
                </div>
            `;
        
        
            document.querySelector("#profile-info").appendChild(participantDiv);
        });                
    } else {
        console.error("Failed to fetch data:", await response.text());
    }
}

document.getElementById("search-button").addEventListener("click", sendToBackend);
