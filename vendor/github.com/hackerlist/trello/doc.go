package trello

/*
curl -s "https://api.trello.com/1/cards/:cardid/checklists?token=XXX&key=XXX" | python -m json.tool

[
    {
        "checkItems": [
            {
                "id": "",
                "name": ""
                "nameData": null,
                "pos": 17334,
                "state": "complete"
        }
        ],
        "id": "52d4778c405dec6d7531e05a",
        "idBoard": "52d4778a405dec6d7531de0d",
        "idCard": "52d4778b405dec6d7531de27",
        "name": "Daily Goals",
        "pos": 16384
}
]

func (cl *Checklist) Checklists() ([]Checklist, error)
*/
