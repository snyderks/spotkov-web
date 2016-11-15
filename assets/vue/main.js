Vue.component('all-songs', {
  props: ['song'],
  template: '<div>{{song.Title}} - {{song.Artist}}</div>',
  replace: true
})
Vue.component('spotify-login', {
  template: '<button class="btn login-btn" v-on:click="spotifyLogin">Login to Spotify</button>',
  replace: true,
  methods: {
    spotifyLogin: function () {
      $.ajax({
        url: 'http://localhost:8080/api/spotifyLoginUrl',
        type: 'GET',
        dataType: 'json'
      })
      .done(function(data) {
        console.log(data)
        if (data.URL !== undefined) {
          if (data.URL.match(/https:\/\/accounts.spotify.com\/authorize?/) != null) {
            window.location = data.URL
          }
        } else {
          fail()
        }
        
      })
      .fail(function(data) {
        console.log("error")
        console.log(data)
      })
    }
  }
})
Vue.component('spotify-test', {
  data: function () {
    return {
      userId: "",
      songs: []
    }
  },
  template: '<div><button class="btn" v-on:click="spotifyTest">Test Spotify Authentication</button>\
             <br/><p v-for="song in songs" v-bind:song="song">{{song.Title}} - {{song.Artist}}</p></div>',
  methods: {
    spotifyTest: function () {
      var comp = this
      if (localStorage.getItem("access_token") !== undefined) {
        var token = {}
        token.access_token = localStorage.getItem("access_token")
        token.expiry = localStorage.getItem("expiry")
        token.refresh_token = localStorage.getItem("refresh_token")
        token.token_type = localStorage.getItem("token_type")

        var request = {}
        request.length = 20
        request.title = "Come To Me"
        request.artist = "The Goo Goo Dolls"
        request.token = token
        request.lastFmUsername = "snyderks"
        request = JSON.stringify(request)
        $.ajax({
        url: 'http://localhost:8080/api/getPlaylist',
        type: 'POST',
        dataType: 'json',
        data: request
      })
      .done(function(data) {
        comp.songs = data
        console.log(data)
      })
      .fail(function(data) {
        console.log("error")
        console.log(data)
      })
      }
    }
  }
})

var app = new Vue({
  el: '#app',
  data: {
    songs: []
  },
  methods: {
    getSongs: function() {
      var inst = this
      $.ajax({
        url: 'http://localhost:8080/api/getLastFMSongs/snyderks',
        type: 'GET',
        dataType: 'json',
      })
      .done(function(data) {
        console.log(data)
        inst.songs = data
      })
      .fail(function(data) {
        console.log("error")
        console.log(data)
      })
    }
  }
})
