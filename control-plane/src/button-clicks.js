// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import VALUES from './values.js'
import { makeTable, nextPage, prevPage } from './jobs-functions.js';
import { query, where, collection, orderBy, limit, Timestamp, collectionGroup } from 'firebase/firestore';

// BUTTON/CLICK FUNCTIONS

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

// when you press enter on search refresh the table with new results
$('#search').keypress(function (e) {
    if (e.which == 13) {
        let searchTerm = $('#search').val()
        VALUES.searchTerm = searchTerm;
        makeTable(VALUES.db, "", true, "search")
        return false;
    }
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