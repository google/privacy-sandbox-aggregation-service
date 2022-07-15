import VALUES from './values.js'
import { deleteJob, makeTable, nextPage, prevPage } from './jobs-functions.js';
import { query, where, collection, orderBy, limit, Timestamp, collectionGroup } from 'firebase/firestore';
import React from 'react';
import { createRoot } from 'react-dom/client';
import Levels from './components/Levels.js';

var currentLevelsOne = VALUES.currentLevelsOne;
var currentLevelsTwo = VALUES.currentLevelsTwo;

if (window.location.href.indexOf('add') != -1) {

    // ADD JOB BUTTON FUNCTIONS

    const firstAggregator = document.querySelector("#aggregator-1-react");
    const firstAggregatorRoot = createRoot(firstAggregator) ;
    const secondAggregator = document.querySelector("#aggregator-2-react");
    const secondAggregatorRoot = createRoot(secondAggregator);

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
            if(window.levelOne == undefined) {
                firstAggregatorRoot.render(<Levels aggregator={1} />)
            } else {
                window.levelOne.addLevel(currentLevel, aggregator)
            }
        } else {
            // increment the current level for aggregator-2
            VALUES.currentLevelsTwo += 1;
            currentLevelsTwo += 1;
            currentLevel = currentLevelsTwo;
            aggregator = 2;
            if(window.levelTwo == undefined) {
                secondAggregatorRoot.render(<Levels aggregator={2} />)
            } else {
                window.levelTwo.addLevel(currentLevel, aggregator)
            }
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

    // TODO - finish implementing add job functionality
    // document.getElementById('addjob').addEventListener('click', () => {
    //     window.location.href = './add';
    // });

    // when a job is clicked open up the logs and change color/icons
    $('#render-table').on('click', '.job', function () {
        let id = $(this).attr('id');
        let infoId = '#' + id + '-info';
        $(infoId).toggle();
        $('#' + id + '-dropdown').html($('#' + id + '-dropdown').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
        let status = $(this).data('status');
        $(this).toggleClass('active-job-' + status);
    });

    // when the header of a log is clicked open up the info with result/message and change color/icons
    $('#render-table').on('click', '.log-header', function () {
        let id = $(this).data('id');
        let infoId = '#' + id + '-info';
        $(infoId).toggleClass('active-log');
        let status = $(this).data('status');
        $('#' + id + '-header i:last-of-type').html($('#' + id + '-header i:last-of-type').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
        $(this).toggleClass('active-job-' + status);
    });

    // when the filter icon is clicked toggle the side panel
    $('#filter-jobs').on("click", function () {
        $('.page-content').css('display', 'flex')
        $('.filter-side-holder').toggle()
    });

    // created and updated cant be true at the same time
    $('#created').on("change", function () {
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

    $('#filter-jobs-button').on("click", function () {
        let status = $('#status').val();
        let created = $('#created').val();
        let updated = $('#updated').val();
        if (status == "" && created == "" && updated == "") {
            // reset the table to original values
            $('.pages span').html(0);
            makeTable(VALUES.db, query(collection(VALUES.db, VALUES.collection), orderBy('created', 'desc'), limit(10)))
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
            VALUES.direction = 0;

            // the different ways the query can be made
            if(status != "" && created == "" && updated == "") {
                filterQuery = query(collectionGroup(VALUES.db, "levels"), where("status", "==", status), orderBy('created', 'desc'), limit(10))
            } else if (status != "" && created != "") {
                filterQuery = query(collectionGroup(VALUES.db, "levels"), where("status", "==", status), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), limit(10))
            } else if (status != "" && updated != "") {
                filterQuery = query(collectionGroup(VALUES.db, "levels"), where("status", "==", status), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), limit(10))
            } else if (updated != "" && created == "") {
                filterQuery = query(collection(VALUES.db, VALUES.collection), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), limit(10))
            } else if (created != "" && updated == "") {
                filterQuery = query(collection(VALUES.db, VALUES.collection), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), limit(10))
            }

            // make the table
            if(filterQuery != null) {
                $('.pages span').html(1);
                makeTable(VALUES.db, filterQuery, true, status)
            }
        }
    })

    // this controls when you click a dropdown item
    $('#render-table').on('click', '.dropdown-setting', function () {
        // TODO - Implement adding label functionality

        // let id = $(this).data('id')
        // let type = $(this).data('type')
        // if (type == "delete" && VALUES.db != null) {
        //     // delete the job
        //     deleteJob(VALUES.db, id)
        //     $("#" + id).remove()
        //     $("#" + id + "-info").remove()
        // } else if (type == "edit") {
        //     // navigate to the update page
        //     window.location.href = "./update?id=" + id
        // }
    })

    // when you press enter on search refresh the table with new results
    $('#search').keypress(function (e) {
        if (e.which == 13) {
            let searchTerm = $('#search').val()
            VALUES.searchTerm = searchTerm;
            makeTable(VALUES.db, "", true, "search")
            return false;
        }
    });

    $('#render-table').on('click', '#delete-job', function() {
        let id = $(this).data('id')
        // delete the job
        deleteJob(VALUES.db, id)
        $("#" + id).remove()
        $("#" + id + "-info").remove()
    });

    // PAGINATION

    // previous button
    $('#prev-page').on("click", function () {
        prevPage()
    });

    // next button
    $('#next-page').on("click", function () {
        nextPage()
    });
}