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
$('#render-content').on('click', '.job', function () {
    let id = $(this).attr('id');
    let infoId = '#' + id + '-info';
    $(infoId).toggle();
    $('#' + id + '-dropdown').html($('#' + id + '-dropdown').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
    let status = $(this).data('status');
    $(this).toggleClass('active-job-' + status);
});

// when the header of a log is clicked open up the info with result/message and change color/icons
$('#render-content').on('click', '.log-header', function () {
    let id = $(this).data('id');
    let infoId = '#' + id + '-info';
    $(infoId).toggleClass('active-log');
    let status = $(this).data('status');
    $('#' + id + '-header i:last-of-type').html($('#' + id + '-header i:last-of-type').html() == 'keyboard_arrow_down' ? 'keyboard_arrow_up' : 'keyboard_arrow_down');
    $(this).toggleClass('active-job-' + status);
});

// when the filter icon is clicked toggle the side panel
$('#render-content').on('click', '#filter-jobs', function () {
    $('.page-content').css('display', 'flex')
    $('.filter-side-holder').toggle()
});

// created and updated cant be true at the same time
$('#render-content').on('change', '#created', function () {
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
$('#render-content').on('change', '#updated', function () {
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
$('#render-content').on('click', '#prev-page', function () {
    prevPage()
});

// next button
$('#render-content').on('click', '#next-page', function () {
    nextPage()
});