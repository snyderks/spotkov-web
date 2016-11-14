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
