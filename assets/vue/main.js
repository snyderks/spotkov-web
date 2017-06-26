var retrieveToken = function() {
  var token = null;
  if (localStorage.getItem("access_token") !== null) {
    token = {};
    token.access_token = localStorage.getItem("access_token");
    token.expiry = localStorage.getItem("expiry");
    token.refresh_token = localStorage.getItem("refresh_token");
    token.token_type = localStorage.getItem("token_type");
  }
  return token;
};

var setToken = function(token) {
  // make sure the token has all required properties.
  if (
    token.access_token !== null &&
    token.expiry !== null &&
    token.refresh_token !== null &&
    token.token_type !== null
  ) {
    localStorage.setItem("access_token", token.access_token);
    localStorage.setItem("expiry", token.expiry);
    localStorage.setItem("refresh_token", token.refresh_token);
    localStorage.setItem("token_type", token.token_type);
  }
};

Vue.component("spotify-login", {
  template:
    '<button class="btn login-btn" v-on:click="spotifyLogin">Login to Spotify</button>',
  replace: true,
  methods: {
    spotifyLogin: function() {
      $.ajax({
        url: "api/spotifyLoginUrl",
        type: "GET",
        dataType: "json"
      }).done(function(data) {
        if (data.URL !== undefined) {
          if (
            data.URL.match(/https:\/\/accounts.spotify.com\/authorize?/) != null
          ) {
            window.location = data.URL;
          }
        } else {
          fail();
        }
      });
    }
  }
});

var app = new Vue({
  el: "#app",
  data: {
    songs: [],
    songName: localStorage.getItem("songName") === null
      ? ""
      : localStorage.getItem("songName"),
    artistName: localStorage.getItem("artistName") === null
      ? ""
      : localStorage.getItem("artistName"),
    lastFMID: localStorage.getItem("lastFMID") === null
      ? ""
      : localStorage.getItem("lastFMID"),
    suggestions: [],
    length: localStorage.getItem("length") === null
      ? "20"
      : localStorage.getItem("length"),
    error: "",
    message: "",
    activity: false
  },
  computed: {
    loggedIn: function() {
      return localStorage.getItem("access_token") !== null;
    },
    songLabel: function() {
      return parseInt(this.length) === 1 ? "song" : "songs";
    },
    playlistGenerated: function() {
      return this.songs.length > 0;
    }
  },
  methods: {
    getSongs: function() {
      this.songs = [];

      /* Input validation */
      var valid = true;
      if (this.songName.length === 0) {
        $(".song-name").addClass("invalid");
        valid = false;
      } else {
        $(".song-name").removeClass("invalid");
        localStorage.setItem("songName", this.songName);
      }

      if (this.artistName.length === 0) {
        $(".artist-name").addClass("invalid");
        valid = false;
      } else {
        $(".artist-name").removeClass("invalid");
        localStorage.setItem("artistName", this.artistName);
      }

      if (this.lastFMID.length === 0) {
        $(".last-fm-id").addClass("invalid");
        valid = false;
      } else {
        $(".last-fm-id").removeClass("invalid");
        localStorage.setItem("lastFMID", this.lastFMID);
      }

      localStorage.setItem("length", this.length);

      if (valid === false) {
        this.error = "Please fill out all fields";
        return;
      }

      /* AJAX call */
      this.error = "";
      this.activity = true;
      var comp = this; // pass this into local to carry into ajax return
      var token = retrieveToken();
      if (token !== null) {
        var request = {};
        request.length = comp.length;
        request.title = comp.songName;
        request.artist = comp.artistName;
        request.token = token;
        request.lastFmUsername = comp.lastFMID;
        request = JSON.stringify(request);

        // set a timer to trigger a message if the request is taking a while
        var timer = $.timer(function() {
          comp.message =
            "Your first playlist might take a while. Please be patient!";
        });
        timer.set({ time: 6000, autostart: true });
        $.ajax({
          url: "api/getPlaylist",
          type: "POST",
          dataType: "json",
          data: request
        })
          .done(function(data) {
            comp.songs = data;
          })
          .fail(function(data) {
            if (data.error !== undefined && data.error !== null) {
              comp.error = data.error;
            } else {
              comp.error = data.responseText;
              if (comp.error === undefined || comp.error.length === 0) {
                comp.error =
                  "An unknown error occurred in processing your request. Please try again later.";
              }
            }
          })
          .always(function() {
            timer.stop();
            comp.activity = false;
            comp.message = "";
          });
      } else {
        this.error =
          "You're currently not logged in to Spotify. Log in and try again.";
      }
    },
    deleteSong: function(songIndex) {
      this.songs.splice(songIndex, 1);
    },
    createPlaylist: function() {
      this.activity = true;
      this.message = "";
      var comp = this;
      var token = retrieveToken();
      if (token !== null) {
        var request = {};
        request.token = token;
        request.playlistName = "Generated by Spotkov";
        request.songs = this.songs;
        request = JSON.stringify(request);
        $.ajax({
          url: "api/createPlaylist",
          type: "POST",
          dataType: "json",
          data: request
        })
          .done(function(data) {
            // attempt to retrieve the updated token.
            if (data !== undefined && data !== null) {
              var tok = data.token;
              if (tok !== undefined && tok !== null) {
                console.log(tok);
                setToken(tok);
              }
            }
            comp.error = "";
            comp.message =
              "Successfully uploaded your playlist to Spotify! Look under the name Generated by Spotkov";
          })
          .fail(function(data) {
            if (data.error !== undefined && data.error !== null) {
              comp.error = data.error;
            } else {
              comp.error =
                "Couldn't connect to Spotify. Please try again later.";
            }
          })
          .always(function() {
            comp.activity = false;
          });
      } else {
        this.error =
          "You're currently not logged in to Spotify. Log in and try again.";
      }
    },
    autocompleteSong: function() {
      var comp = this;
      // don't do anything if the user doesn't have an ID entered
      if (comp.lastFMID != "") {
        var request = {
          s: comp.songName,
          userID: comp.lastFMID
        };
        request = JSON.stringify(request);

        $.ajax({
          url: "api/songMatches",
          type: "POST",
          dataType: "json",
          data: request
        }).done(function(data) {
          comp.suggestions = data.matches;
        });
      }
    },
    autocompleteArtist: function() {
      var comp = this;
      // don't do anything if the user doesn't have an ID entered
      if (comp.lastFMID != "") {
        var request = {
          s: comp.artistName,
          userID: comp.lastFMID
        };
        request = JSON.stringify(request);

        $.ajax({
          url: "api/artistMatches",
          type: "POST",
          dataType: "json",
          data: request
        }).done(function(data) {
          comp.suggestions = data.matches;
        });
      }
    },
    syncSuggestions: function(useTitle, event) {
      // What the user entered in the box
      var s = event.target.value;
      var match = null;
      // Go through all the suggestions and see if something matched
      for (var item in this.suggestions) {
        if (
          (useTitle &&
            s ===
              this.suggestions[item].Title +
                " - " +
                this.suggestions[item].Artist) ||
          (!useTitle &&
            s ===
              this.suggestions[item].Artist +
                " - " +
                this.suggestions[item].Title)
        ) {
          match = this.suggestions[item];
          break;
        }
      }
      if (match != null) {
        this.songName = match.Title;
        this.artistName = match.Artist;
      }
    },
    toggleInstructAnswer: function() {
      $(".instruct-answer").fadeToggle(500);
    }
  }
});
