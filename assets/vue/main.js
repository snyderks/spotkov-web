var retrieveToken = function () {
  var token = null
  if (localStorage.getItem("access_token") !== null) {
     token = {}
     token.access_token = localStorage.getItem("access_token")
     token.expiry = localStorage.getItem("expiry")
     token.refresh_token = localStorage.getItem("refresh_token")
     token.token_type = localStorage.getItem("token_type")
  }
  return token
}

Vue.component('spotify-login', {
  template: '<button class="btn login-btn" v-on:click="spotifyLogin">Login to Spotify</button>',
  replace: true,
  methods: {
    spotifyLogin: function () {
      $.ajax({
        url: 'api/spotifyLoginUrl',
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
    songs: [],
    songName: localStorage.getItem("songName") === null ? "" : localStorage.getItem("songName"),
    artistName: localStorage.getItem("artistName") === null ? "" : localStorage.getItem("artistName"),
    lastFMID: localStorage.getItem("lastFMID") === null ? "" : localStorage.getItem("lastFMID"),
    length: "20",
    error: "",
    message: "",
    activity: false
  },
  computed: {
    loggedIn: function () {
      return localStorage.getItem("access_token") !== null
    },
    songLabel: function () {
      return parseInt(this.length) === 1 ? "song" : "songs"
    },
    playlistGenerated: function () {
      return this.songs.length > 0
    },
  },
  methods: {
    getSongs: function () {
      this.songs = [];

      /* Input validation */
      var valid = true;
      if (this.songName.length === 0) {
        $(".song-name").addClass("invalid")
        valid = false
      } else {
        $(".song-name").removeClass("invalid")
        localStorage.setItem("songName", this.songName)
      }

      if (this.artistName.length === 0) {
        $(".artist-name").addClass("invalid")
        valid = false
      } else {
        $(".artist-name").removeClass("invalid")
        localStorage.setItem("artistName", this.artistName)
      }

      if (this.lastFMID.length === 0) {
        $(".last-fm-id").addClass("invalid")
        valid = false
      } else {
        $(".last-fm-id").removeClass("invalid")
        localStorage.setItem("lastFMID", this.lastFMID)
      }

      if (valid === false) {
        this.error = "Please fill out all fields"
        return
      }

      /* AJAX call */
      this.error = ""
      this.activity = true
      var comp = this // pass this into local to carry into ajax return
      var token = retrieveToken()
      if (token !== null) {
        var request = {}
        request.length = comp.length
        request.title = comp.songName
        request.artist = comp.artistName
        request.token = token
        request.lastFmUsername = comp.lastFMID
        request = JSON.stringify(request)

        // set a timer to trigger a message if the request is taking a while
        var timer = $.timer(function() {
          comp.message = "Your first playlist might take a while. Please be patient!";
        })
        timer.set({ time: 5000, autostart: true })
        $.ajax({
          url: 'api/getPlaylist',
          type: 'POST',
          dataType: 'json',
          data: request
        })
        .done(function(data) {
          comp.songs = data
          console.log(data)
        })
        .fail(function(data) {
          comp.error = data.responseText
          if (comp.error === undefined || comp.error.length === 0) {
            comp.error = "An unknown error occurred in processing your request. Please try again later."
          }
        })
        .always(function() {
          timer.stop()
          comp.activity = false
          comp.message = ""
        })
        
      } else {
        this.error = "You're currently not logged in to Spotify. Log in and try again."
      }
    },
    deleteSong: function(songIndex) {
      this.songs.splice(songIndex, 1)
    },
    moveSong: function (from, to) {
      var toMove = this.songs[from]
      this.songs.splice(from, 1)
      this.songs.splice(to, 0, toMove)
    },
    createPlaylist: function () {
      this.activity = true
      this.message = ""
      var comp = this
      var token = retrieveToken()
      if (token !== null) {
        var request = {}
        request.token = token
        request.playlistName = "Generated by Spotkov"
        request.songs = this.songs
        request = JSON.stringify(request)
        $.ajax({
          url: 'api/createPlaylist',
          type: 'POST',
          data: request
        })
        .done(function () {
          comp.error = ""
          comp.message = "Successfully uploaded your playlist to Spotify! Look under the name Generated by Spotkov"
        })
        .fail(function () {
          comp.error = "Couldn't connect to Spotify. Please try again later."
        })
        .always(function() {
          comp.activity = false
        })
      } else {
        this.error = "You're currently not logged in to Spotify. Log in and try again."
      }
    }
  }
})

app.$watch(
  function () {
    return app.playlistGenerated && document.getElementById("playlist") !== null
  },
  function (newVal, oldVal) {
    sortable = Sortable.create(document.getElementById("playlist"), {
      animation: 150, // animation speed for movement
      onEnd: function (/**Event*/evt) {
        app.moveSong(evt.oldIndex, evt.newIndex);
    }
    })
  }
)
