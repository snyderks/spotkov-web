$(document).ready(function() {
  var nativedatalist =
    !!("list" in document.createElement("input")) &&
    !!(document.createElement("datalist") && window.HTMLDataListElement);

  if (!nativedatalist) {
    $("input[list]").each(function() {
      var availableTags = $("#" + $(this).attr("list"))
        .find("option")
        .map(function() {
          return this.value;
        })
        .get();
      $(this).autocomplete({ source: availableTags });
    });
  }
});
