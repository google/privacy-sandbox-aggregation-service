import VALUES from './values.js'
import { deleteJob, makeTable, nextPage, prevPage, addLevel } from './jobs-functions.js';
import { query, where, collection, orderBy, limit, Timestamp } from 'firebase/firestore';

var currentLevelsOne = VALUES.currentLevelsOne;
var currentLevelsTwo = VALUES.currentLevelsTwo;

if (window.location.href.indexOf('add') != -1) {

    // ADD JOB BUTTON FUNCTIONS

    // return home button
    document.getElementById('returnhome').addEventListener('click', () => {
        window.location.href = '/';
    });

    // when the level header is clicked show or hide the text fields
    $('.levels').on('click', '.level-header', function (e) {
        if ($(e.target).hasClass('remove-aggregator') == false) {
            let id = $(this).data('id');
            let infoId = '#' + id + '-info';
            // toggle the info section filled with the text fields
            $(infoId).toggle();
            // update the arrow at the end
            $('#' + id + '-header i:last-of-type').html($('#' + id + ' i:last-of-type').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
            // remove or add active-level which changes the color
            $(this).toggleClass('active-level');
        }
    });

    // add the new html for the level
    $('.add-level').on('click', function() {
        let currentLevel = 0;
        let id = $(this).attr('id');
        let aggregator = 0;

        if (id == "add-level-1") {
            // increment the current level for aggregator-1
            VALUES.currentLevelsOne += 1;
            currentLevelsOne += 1;
            currentLevel = currentLevelsOne;
            aggregator = 1;
        } else {
            // increment the current level for aggregator-2
            VALUES.currentLevelsTwo += 1;
            currentLevelsTwo += 1;
            currentLevel = currentLevelsTwo;
            aggregator = 2;
        }

        let appendHtml = addLevel(currentLevel, aggregator);

        // add html to respective aggregator
        if (id == "add-level-1") {
            $('#aggregator-1-levels').append(appendHtml);
        } else {
            $('#aggregator-2-levels').append(appendHtml);
        }
    });

    // TODO - update the value attribute so it doesn't remove the previous value when the html gets updated.
    $('.levels').on('click', '.remove-aggregator', function (e) {
        let id = $(this).data('id');
        // split the id to get the aggregator number and the level number
        let removeValues = id.split("-")
        let aggregatorNumber = removeValues[1];
        let levelNumber = removeValues[3];
        let currentLevel = 0;
        if (aggregatorNumber == 1) {
            currentLevel = currentLevelsOne
        } else {
            currentLevel = currentLevelsTwo
        }

        // remove the level from html tree
        $('#' + id).remove();

        // replace the html with the new level number
        for (let i = levelNumber; i < currentLevel + 1; i++) {
            let html = $('#aggregator-' + aggregatorNumber + '-level-' + i).html();
            if (html != null) {
                // replace the html with the new level number
                $('#aggregator-' + aggregatorNumber + '-level-' + i).html(html.replaceAll('aggregator-' + aggregatorNumber + '-level-' + i, 'aggregator-' + aggregatorNumber + '-level-' + (i - 1)));
                // change the span to show the new level number
                $('#aggregator-' + aggregatorNumber + '-level-' + (i - 1) + '-header span').html('Level ' + (i - 1));
                // update the containers id as well
                $('#aggregator-' + aggregatorNumber + '-level-' + i).attr('id', 'aggregator-' + aggregatorNumber + '-level-' + (i - 1));
            }
        }

        // decrement the level count
        if (aggregatorNumber == 1) {
            currentLevelsOne -= 1;
            VALUES.currentLevelsOne -= 1;
        } else {
            currentLevelsTwo -= 1;
            VALUES.currentLevelsTwo -= 1;
        }
    });
} else if(window.location.href.indexOf('update') != -1) {

    // UPDATE FUNCTIONS

     // return home button
    document.getElementById('returnhome').addEventListener('click', () => {
        window.location.href = '/';
    });

    // when the level header is clicked show or hide the text fields
    $('.levels').on('click', '.level-header', function (e) {
        let id = $(this).data('id');
        let infoId = '#' + id + '-info';
        $(infoId).toggle();
        $('#' + id + '-header i:last-of-type').html($('#' + id + ' i:last-of-type').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
        $(this).toggleClass('active-level');
    });
    
} else {

    // HOMEPAGE FUNCTIONS

    // add job button
    document.getElementById('addjob').addEventListener('click', () => {
        window.location.href = './add';
    });

    // when a job is clicked open up the logs and change color/icons
    $('#aggregation-jobs').on('click', '.job', function () {
        let id = $(this).attr('id');
        let infoId = '#' + id + '-info';
        $(infoId).toggle();
        $('#' + id + '-dropdown').html($('#' + id + '-dropdown').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
        let status = $(this).data('status');
        $(this).toggleClass('active-job-' + status);
    });

    // when the header of a log is clicked open up the info with result/message and change color/icons
    $('#aggregation-jobs').on('click', '.log-header', function () {
        let id = $(this).data('id');
        let infoId = '#' + id + '-info';
        $(infoId).toggle();
        let status = $(this).data('status');
        $('#' + id + '-header i:last-of-type').html($('#' + id + '-header i:last-of-type').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
        $(this).toggleClass('active-job-' + status);
    });

    // when the filter icon is clicked toggle the side panel
    $('#filter-jobs').click(function () {
        $('.page-content').css('display', 'flex')
        $('.filter-side-holder').toggle()
    });

    // created and updated cant be true at the same time
    $('#created').change(function () {
        if ($(this).val() != "" && $('#updated').val() != "") {
            // error bc firebase cant do two range sorts
            $(this).parent().addClass('is-invalid')
            $('#updated').parent().addClass('is-invalid')
            $('#updated-created-error').show()
        } else {
            // good
            $(this).parent().removeClass('is-invalid')
            $('#updated').parent().removeClass('is-invalid')
            $('#updated-created-error').hide()
        }
    })

    // created and updated can't be true at the same time
    $('#updated').on('change', function () {
        if ($(this).val() != "" && $('#created').val() != "") {
            // error bc firebase cant do two range sorts
            $(this).parent().addClass('is-invalid')
            $('#created').parent().addClass('is-invalid')
            $('#updated-created-error').show()
        } else {
            // good
            $(this).parent().removeClass('is-invalid')
            $('#created').parent().removeClass('is-invalid')
            $('#updated-created-error').hide()
        }
    })
    $('#filter-jobs-button').click(function () {
        let status = $('#status').val();
        let created = $('#created').val();
        let updated = $('#updated').val();
        if (status == "" && created == "" && updated == "") {
            // reset the table to original values
            makeTable(VALUES.db, query(collection(VALUES.db, "jobs-test"), orderBy('created', 'desc'), limit(10)))
            $('.filter-side-holder').hide()
        } else {
            let filterQuery = null;
            let createdTimestamp = null;
            let updatedTimestamp = null;
            if(created != "") {
                // get the firebase timestamp so it can compare dates
                let date = new Date();
                date.setHours(date.getHours() - created)
                createdTimestamp = Timestamp.fromDate(date)
            }
            if(updated != "") {
                // get the firebase timestamp so it can compare dates
                let date = new Date();
                date.setHours(date.getHours() - updated)
                updatedTimestamp = Timestamp.fromDate(date)
            }

            VALUES.status = status;
            VALUES.createdTimestamp = createdTimestamp;
            VALUES.updatedTimestamp = updatedTimestamp;

            // the different ways the query can be made
            if(status != "" && created == "" && updated == "") {
                filterQuery = query(collection(VALUES.db, "jobs-test"), where("overrallStatus", "==", status), orderBy('created', 'desc'), limit(10))
            } else if (status != "" && created != "") {
                filterQuery = query(collection(VALUES.db, "jobs-test"), where("overrallStatus", "==", status), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), limit(10))
            } else if (status != "" && updated != "") {
                filterQuery = query(collection(VALUES.db, "jobs-test"), where("overrallStatus", "==", status), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), limit(10))
            } else if (updated != "" && created == "") {
                filterQuery = query(collection(VALUES.db, "jobs-test"), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), limit(10))
            } else if (created != "" && updated == "") {
                filterQuery = query(collection(VALUES.db, "jobs-test"), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), limit(10))
            }

            // make the table
            if(filterQuery != null) {
                makeTable(VALUES.db, filterQuery, true)
            }
        }
    })

    // this controls when you click a dropdown item
    $('#aggregation-jobs').on('click', '.dropdown-setting', function () {
        let id = $(this).data('id')
        let type = $(this).data('type')
        if (type == "delete" && VALUES.db != null) {
            // delete the job
            deleteJob(VALUES.db, id)
            $("#" + id).remove()
            $("#" + id + "-info").remove()
        } else if (type == "edit") {
            // navigate to the update page
            window.location.href = "./update?id=" + id
        }
    })

    // when you press enter on search refresh the table with new results
    $('#search').keypress(function (e) {
        if (e.which == 13) {
            let searchTerm = $('#search').val()
            VALUES.searchTerm = searchTerm;
            let thequery = query(collection(VALUES.db, "jobs-test"), where('name', ">=", searchTerm), where("name", "<", searchTerm + 'z'), orderBy('name', 'desc'), limit(10))
            makeTable(VALUES.db, thequery, true)
            return false;
        }
    });

    // PAGINATION

    // previous button
    $('#prev-page').click(function () {
        prevPage()
    });

    // next button
    $('#next-page').click(function () {
        nextPage()
    });
}