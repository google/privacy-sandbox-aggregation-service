import { doc, updateDoc, setDoc, Timestamp, getDocs, getDoc, collection, deleteDoc, query, startAfter, orderBy, limit, startAt, where } from "firebase/firestore";
import { v4 as uuidv4 } from 'uuid';
import VALUES from './values.js'
import React from 'react';
import { createRoot } from 'react-dom/client';
import Jobs from './components/Jobs';
import UpdateLevel from "./components/UpdateLevel.js";

let currentHref = window.location.href;

// For the jobs table
const container = document.querySelector('#aggregation-jobs tbody');
const tableRoot = currentHref.indexOf('update') == -1 && currentHref.indexOf('add') == -1 ? createRoot(container) : null;

// For update levels
const firstAggregator = document.querySelector("#aggregator-1-levels");
const firstAggregatorRoot = currentHref.indexOf('update') != -1 ? createRoot(firstAggregator) : null;
const secondAggregator = document.querySelector("#aggregator-2-levels");
const secondAggregatorRoot = currentHref.indexOf('update') != -1 ? createRoot(secondAggregator) : null;

// ADD Job function
export async function addJob(db) {
    // created the uuidv4 id for job
    const jobId = uuidv4();
    const jobDoc = doc(db, "jobs", jobId);
    // get the overall status of the job
    const overrallStatus = getOverrallStatusFromInput(VALUES.currentLevelsOne + 1, VALUES.currentLevelsTwo + 1)

    // enter into the jobs doc
    await setDoc(jobDoc, {
        overrallStatus: overrallStatus,
        created: Timestamp.now(),
        updated: Timestamp.now(),
    });

    // created the collections and enter the values into the db
    await enterIntoDb(db, jobId, VALUES.currentLevelsOne + 1, 'aggregator-1')
    await enterIntoDb(db, jobId, VALUES.currentLevelsTwo + 1, 'aggregator-2')

    // navigate back to home and show success message
    window.location.href = '/?job=success';
}

// update job function
export async function updateJob(db, jobId) {
    const jobDoc = doc(db, "jobs", jobId);
    // get the overall status of the job
    const overrallStatus = getOverrallStatusFromInput(VALUES.currentLevelsOne, VALUES.currentLevelsTwo);

    // update the name, status, and last updated of job doc
    await updateDoc(jobDoc, {
        overrallStatus: overrallStatus,
        updated: Timestamp.now(),
    });

    // update the levels 
    await updateLevels(db, jobId, VALUES.currentLevelsOne, 'aggregator-1')
    await updateLevels(db, jobId, VALUES.currentLevelsTwo, 'aggregator-2')

    // return to home with edit success
    window.location.href = '/?job=editsuccess';
}

// make sure that the job name is filled
export function validateFields() {
    if ($("#job-name-field").val() != "") {
        return true;
    }
    return false;
}

// build the table using the query passed
export async function makeTable(db, thequery, first) {
    // show loader
    $('#p2').show();
    // get all the jobs returned from the query
    let values = await getJobs(db, thequery)
    // values[0] is the jobs, values[1] is the last job, values[2] is the first job
    let jobs = values[0];
    // set values for pagination
    setValues(values[1], values[2], first)

    // if jobs array is greater than 0, make the table otherwise show no jobs
    if (jobs.length > 0) {
        // check if the user can go to the next page
        canAdvance(jobs.length)
        // reset the table
        $('#aggregation-jobs tbody').html("")
        // hide the loader
        $('#p2').hide();
        // set the html of the table
        setHtml(jobs)
    } else {
        // basic no jobs response
        $('#p2').hide();

        let newHtml = '<td colspan=5><h5 style="text-align: center;">No Jobs</h5></td>';
        $('#aggregation-jobs tbody').html(newHtml)
    }
}

// delete the job with jobId
export async function deleteJob(db, job) {
    await deleteDoc(doc(db, "jobs", job));
}

// go to next page
export function nextPage() {
    let nextQuery = null;

    // make sure that the last query is not null and user can advance
    if(VALUES.last != null && VALUES.canAdvance) {
        // get the specific query and start after the last value in the current table
        nextQuery = getQuery(startAfter(VALUES.last))

        // if query is found then make the table
        if(nextQuery != false) {
            makeTable(VALUES.db, nextQuery, false)
            // increment the span holding the page number
            $('.pages span').html(parseInt($('.pages span').html()) + 1);
        }
    }
}

export function prevPage() {
    let prevQuery = null;
    // make sure that prevFirst exists and that the page number is not 0
    if(VALUES.prevFirst != null && parseInt($('.pages span').html()) != 0) {
        // get the query, but this time start at the prev first
        prevQuery = getQuery(startAt(VALUES.prevFirst))

        // if the query is found, then make the table
        if(prevQuery != false) {
            makeTable(VALUES.db, prevQuery, false)
            // increment the span holding the page number
            $('.pages span').html(parseInt($('.pages span').html()) - 1);
        }
    }
}

export async function initUpdatePage(db, jobId) {
    // get data from firestore
    const jobData = await getJobData(db, jobId)
    // validate data
    if(jobData == null) {
        window.location.href = '/';
    }
    // fill in fields
    fillInUpdateFields(jobData)
}

export function addLevel(aggregator, currentLevel) {
    let appendHtml = `
        <div id="aggregator-${aggregator}-level-${currentLevel}" class="level">
            <div id="aggregator-${aggregator}-level-${currentLevel}-header" class="level-header active-level" data-id="aggregator-${aggregator}-level-${currentLevel}">
                <span>Level ${currentLevel}</span>
                <i class="material-icons remove-aggregator" data-id="aggregator-${aggregator}-level-${currentLevel}">close</i><i class="material-icons">keyboard_arrow_down</i>
            </div>
            <div id="aggregator-${aggregator}-level-${currentLevel}-info" class="mdl-grid level-info">
                <div class="mdl-cell mdl-cell--12-col aggregator-${aggregator}-level-${currentLevel}">
                    <div class="mdl-textfield mdl-js-textfield">
                        <textarea class="mdl-textfield__input" type="text" rows="3"
                            id="aggregator-${aggregator}-level-${currentLevel}-message-field"></textarea>
                        <label class="mdl-textfield__label" for="aggregator-${aggregator}-level-${currentLevel}-message-field">Message</label>
                    </div>
                    <br>
                    <div class="mdl-textfield mdl-js-textfield">
                        <textarea class="mdl-textfield__input" type="text" rows="3"
                            id="aggregator-${aggregator}-level-${currentLevel}-result-field"></textarea>
                        <label class="mdl-textfield__label" for="aggregator-${aggregator}-level-${currentLevel}-result-field">Result</label>
                    </div>
                    <br>
                    <h6>Current Status</h6>
                    <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-${aggregator}-level-${currentLevel}-option-1">
                        <input type="radio" id="aggregator-${aggregator}-level-${currentLevel}-option-1" class="mdl-radio__button" name="aggregator-${aggregator}-level-${currentLevel}-options"
                            value="Scheduled" checked>
                        <span class="mdl-radio__label">Scheduled</span>
                    </label>
                    <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-${aggregator}-level-${currentLevel}-option-2">
                        <input type="radio" id="aggregator-${aggregator}-level-${currentLevel}-option-2" class="mdl-radio__button" name="aggregator-${aggregator}-level-${currentLevel}-options"
                            value="Running">
                        <span class="mdl-radio__label">Running</span>
                    </label>
                    <br>
                    <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-${aggregator}-level-${currentLevel}-option-3">
                        <input type="radio" id="aggregator-${aggregator}-level-${currentLevel}-option-3" class="mdl-radio__button" name="aggregator-${aggregator}-level-${currentLevel}-options"
                            value="Finished">
                        <span class="mdl-radio__label">Finished</span>
                    </label>
                    <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-${aggregator}-level-${currentLevel}-option-4">
                        <input type="radio" id="aggregator-${aggregator}-level-${currentLevel}-option-4" class="mdl-radio__button" name="aggregator-${aggregator}-level-${currentLevel}-options"
                            value="Failed">
                        <span class="mdl-radio__label">Failed</span>
                    </label>
                </div>
            </div>
        </div>
    `;

    return appendHtml;
}

// function to get a specific document's information
async function getJobData(db, jobId) {
    let docSnap = await getDoc(doc(VALUES.db, "jobs", jobId));
    // if document exists get name and level information, else return null
    if(docSnap.exists()) {
        const jobName = docSnap.data().name;
        const aggregatorOneLevels = await getLevels(db, jobId, 'aggregator-1', [])
        const aggregatorTwoLevels = await getLevels(db, jobId, 'aggregator-2', [])
        return [jobName, aggregatorOneLevels, aggregatorTwoLevels]
    }
    return null;
}

// fill in the update page fields
function fillInUpdateFields(jobData) {
    const jobName = jobData[0];
    const aggregatorOneLevels = jobData[1];
    const aggregatorTwoLevels = jobData[2];
    VALUES.currentLevelsOne = aggregatorOneLevels.length;
    VALUES.currentLevelsTwo = aggregatorTwoLevels.length;

    // fill in job field
    $('#job-name-field').val(jobName)
    // field starts with invalid so remove that class
    $('#job-name-field').parent().removeClass('is-invalid')
    // is-dirty indicates that field has a value
    $('#job-name-field').parent().addClass('is-dirty')
    
    firstAggregatorRoot.render(<UpdateLevel levels={aggregatorOneLevels} aggregator={1} />)
    secondAggregatorRoot.render(<UpdateLevel levels={aggregatorTwoLevels} aggregator={2} />)
}

// get the specific query for either prevPage or nextPage and startAt or startAfter marker
function getQuery(marker) {
    // cleans the code later on
    const status = VALUES.status;
    const createdTimestamp = VALUES.createdTimestamp;
    const updatedTimestamp = VALUES.updatedTimestamp;
    const searchTerm = VALUES.searchTerm;
    const db = VALUES.db

    if(searchTerm == "") {
        if (status == "" && createdTimestamp == "" && updatedTimestamp == "") {
            return query(collection(db, "jobs"), orderBy('created', 'desc'), marker, limit(10))
        } else if(status != "" && createdTimestamp == "" && updated == "") {
            return query(collection(db, "jobs"), where("overrallStatus", "==", status), orderBy('created', 'desc'), marker, limit(10))
        } else if (status != "" && createdTimestamp != "") {
            return query(collection(db, "jobs"), where("overrallStatus", "==", status), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), marker, limit(10))
        } else if (status != "" && updatedTimestamp != "") {
            return query(collection(db, "jobs"), where("overrallStatus", "==", status), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), marker, limit(10))
        } else if (updatedTimestamp != "" && createdTimestamp == "") {
            return query(collection(db, "jobs"), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), marker, limit(10))
        } else if (createdTimestamp != "" && updatedTimestamp == "") {
            return query(collection(db, "jobs"), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), marker, limit(10))
        }
        return false
    } else {
        return query(collection(db, "jobs"), where('name', ">=", searchTerm), where("name", "<", searchTerm + 'z'), orderBy('name', 'desc'), marker, limit(10))
    }
}

// get the overrall status so that it is easier to grab
function getOverrallStatusFromInput(levelsOne, levelsTwo) {
    // the counts of the different statuses
    const statusCount = { "Running": 0, "Scheduled": 0, "Finished": 0 };

    for (let i = 0; i <  levelsOne; i++) {
        if ($("input[type=radio][name=aggregator-1-level-" + i + "-options]:checked").val() == "Failed") {
            // if status is failed the job failed
            return "Failed";
        }
        // update status count
        statusCount[$("input[type=radio][name=aggregator-1-level-" + i + "-options]:checked").val()] += 1;
    }
    for (let i = 0; i < levelsTwo; i++) {
        if ($("input[type=radio][name=aggregator-2-level-" + i + "-options]:checked").val() == "Failed") {
            // if status is failed the job failed
            return "Failed";
        }
        // update status count
        statusCount[$("input[type=radio][name=aggregator-2-level-" + i + "-options]:checked").val()] += 1;
    }

    return decideStatus(statusCount);
}

function getOverrallStatus(logs) {
    const statusCount = { "Running": 0, "Scheduled": 0, "Finished": 0 };
    for(let i=0; i < logs.length; i++) {
        statusCount[logs[i].status] += 1;
    }

    return decideStatus(statusCount);
}

function decideStatus(statusCount) {
    if (statusCount['Running'] == 0 && statusCount['Scheduled'] == 0 && statusCount['Finished'] > 0) {
        // if all of them are finished, job is done
        return "Finished";
    } else if (statusCount['Running'] > 0) {
        // if any of the jobs are running, then the job is running
        return "Running";
    } else if (statusCount['Running'] == 0 && statusCount['Scheduled'] > 0) {
        // if no jobs running, then the status is scheduled
        return "Scheduled";
    }
    // return scheduled if not in any of these
    return "Scheduled";
}

// a nice function to calculate how long ago the last update was
export function timeSince(date) {

    const seconds = Math.floor((new Date() - date) / 1000);

    let interval = seconds / 31536000;

    if (interval > 1) {
        return Math.floor(interval) + " years";
    }
    interval = seconds / 2592000;
    if (interval > 1) {
        return Math.floor(interval) + " months";
    }
    interval = seconds / 86400;
    if (interval > 1) {
        return Math.floor(interval) + " days";
    }
    interval = seconds / 3600;
    if (interval > 1) {
        return Math.floor(interval) + " hours";
    }
    interval = seconds / 60;
    if (interval > 1) {
        return Math.floor(interval) + " minutes";
    }
    return Math.floor(seconds) + " seconds";
}


// get the 10 jobs that are requested and the last document
async function getJobs(db, thequery) {
    let jobs = [];
    const querySnapshot = await getDocs(thequery);
    // populate the jobs array
    jobs = await populateJobs(db, jobs, querySnapshot);
    // get the last document. used for pagination
    const last = querySnapshot.docs[querySnapshot.docs.length-1];
    // get the first document. used for pagination
    const first = querySnapshot.docs[0]
    return [jobs, last, first];
}


// populate the jobs variable using an async method and in parallel
async function populateJobs(db, jobs, querySnapshot) {
    await Promise.all(querySnapshot.docs.map(async (doc) => {
        let job = {};
        job.id = doc.id;
        let vals = doc.data();
        job.created = vals.created;
        job.updated = vals.updated;
        let levelLogs = [];
        // fill in the levelLogs field with the jobs logs 
        levelLogs = await getLevels(db, doc.id, 'aggregator-1', levelLogs)
        levelLogs = await getLevels(db, doc.id, 'aggregator-2', levelLogs)
        job.logs = levelLogs;
        if(vals.overrallStatus != null) {
            // job finished or failed
            job.status = vals.overrallStatus;
        } else {
            // job scheduled or running or top level document hasn't been updated
            job.status = getOverrallStatus(levelLogs)
        }
        jobs.push(job);
    }));

    return jobs;
}

// a nice function to get the logs for a given document and aggregator
// and fill in the levelLogs array and return it back
async function getLevels(db, docId, aggregator, levelLogs) {
    const aggregatorSnapshot = await getDocs(collection(db, "jobs", docId, aggregator));
    aggregatorSnapshot.forEach((doc) => {
        let log = {};
        log.level = aggregator+"-"+doc.id;
        let vals = doc.data();
        log.created = vals.created;
        log.message = vals.message;
        log.result = vals.result;
        log.status = vals.status;
        log.total_levels = vals.total_levels;
        log.updated = vals.updated;
        levelLogs.push(log)
    });
    return levelLogs;
}

// change a firestore timestamp to a date and then format it
export function formatTime(timestamp, updated) {
    let date = timestamp.toDate();
    // if updated get the amount of time it has been since the timestamp
    if (updated) {
        return timeSince(date) + " ago";
    }
    let day = date.getDate();
    let month = date.getMonth() + 1;
    let year = date.getFullYear();
    return month + "/" + day + "/" + year
}

// enter all the level documents into the db
async function enterIntoDb(db, jobId, levels, aggregator) {
    for (let i = 0; i < levels; i++) {
        const levelDoc = doc(db, "jobs", jobId, aggregator, "level-" + i);
        await setDoc(levelDoc, {
            created: Timestamp.now(),
            updated: Timestamp.now(),
            status: $("input[type=radio][name="+aggregator+"-level-" + i + "-options]:checked").val(),
            total_levels: levels,
            result: $("#"+aggregator+"-level-" + i + "-result-field").val(),
            message: $("#"+aggregator+"-level-" + i + "-message-field").val()
        });
    }
}

// update all the level documents
async function updateLevels(db, jobId, levels, aggregator) {
    for (let i = 0; i < levels; i++) {
        const levelDoc = doc(db, "jobs", jobId, aggregator, "level-" + i);
        await updateDoc(levelDoc, {
            updated: Timestamp.now(),
            status: $("input[type=radio][name="+aggregator+"-level-" + i + "-options]:checked").val(),
            result: $("#"+aggregator+"-level-" + i + "-result-field").val(),
            message: $("#"+aggregator+"-level-" + i + "-message-field").val()
        });
    }
}

function setValues(last, first, set) {
    VALUES.last = last;
    // if it is the very first page then prevfirst is equal to first
    if(set) {
        VALUES.prevFirst = first
    } else {
        VALUES.prevFirst = VALUES.first;
    }
    VALUES.first = first;
}

function canAdvance(length) {
    // if it is less than 10, that means that there are no more jobs left in query
    // so pagination should stop
    if(length < 10) {
        VALUES.canAdvance = false;
    } else {
        VALUES.canAdvance = true;
    }
}

function setHtml(jobs) {
    tableRoot.render(<Jobs jobs={jobs} />);
}