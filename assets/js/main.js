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
      .fail(function() {
        console.log("error")
      })
      .always(function() {
        console.log("complete")
      })

    }
  }
})
