package wxchannels_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wxchannels "wx_channel/internal/adapter/wxchannels"
	wxchannelspkg "wx_channel/pkg/scraper/wxchannels"
)

// liveFeedJSON is a ChannelsObject live feed fixture.  It contains the fields
// returned by the feed profile API (fetched on the live page), including
// anchorContact which carries the streamer's display info.
const liveFeedJSON = `{
	"id": "14962698468287781449",
	"nickname": "小玉来了哦",
	"username": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
	"objectDesc": {
		"description": "谁可以无缘无故给我刷个岛",
		"media": [
			{
				"url": "",
				"thumbUrl": "",
				"mediaType": 4,
				"videoPlayLen": 0,
				"width": 0,
				"height": 0,
				"coverUrl": "",
				"decodeKey": "",
				"fileSize": 0
			}
		],
		"mediaType": 4
	},
	"contact": {
		"username": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
		"nickname": "小玉来了哦",
		"headUrl": "https://wx.qlogo.cn/finderhead/contact_avatar.jpg"
	},
	"liveInfo": {
		"anchorStatusFlag": "5908472000",
		"switchFlag": 4607,
		"sourceType": 0,
		"micSetting": {
			"settingFlag": 0,
			"settingSwitchFlag": 4
		},
		"lotterySetting": {
			"settingFlag": 0,
			"attendType": 3
		},
		"liveCoverImgs": [
			{
				"url": "https://example.com/live_cover.jpg"
			}
		]
	},
	"anchorContact": {
		"username": "anchor_user_123",
		"nickname": "主播昵称",
		"headUrl": "https://wx.qlogo.cn/finderhead/anchor_avatar.jpg",
		"coverImgUrl": "https://example.com/anchor_cover.jpg",
		"signature": "欢迎来到我的直播间"
	},
	"createtime": 1785075244,
	"objectNonceId": "live_nonce_123_0",
	"liveCookies": "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",
	"liveSdkInfo": {
		"sdkAppid": 1400419933,
		"sdkUserId": "o9hHn5TBuFNJvai5p-_5eOWlbabk",
		"sdkLiveId": 221795444,
		"sdkRoleId": 1,
		"sdkUserSig": "ZUp3dHpkMEtna0FRQmVCMzJkdEtWdDNKVnVoR1NMS2tBZzB2dzhYTkJzdC14WXplUFZNdjV6dUhNeC1pdTU3U3lwS1lSRk1vV1k0M1JqS3Q4WTRqWi15eFQ4RzNHdnQwYUVPRWZIVURlUTZlSWhUSjNLKmlKTXh6aklpcE1rcVp5cm11VDRuc2Npemw0QUNnVVVvbnJmSDFOMk1EMUZnenh1WVZqSWQzWVBUZW9nYUw3VERKM2hBVUYqZG9NUzJXbFIwQk9HN1hTQkVYWFMtNGRVdSpQMVhuT1AwXw==",
		"sdkPrivateMapKey": "ZUp3dGpkMVN3akFVaE44bHR5b21OQUhhR1M2aVdINktaYVFWNjVXVGtGUmpFOGkwYVVFYzM5MEN2ZG1aOCozWjNWKlFMcE5lSTBzUWdINFBndHZMcllUY09aV3JDOTc3WDdNZFNSLXFNRjQwVEJGNzkwSGs2azF6eG92dXZ4SUZzMVlKRUNBTUlVYSo3M2xYUng2dEtpVUlSb1BXdUNLblRBdlFjRVRnY0lBeDdpclVaN3NWOWRQSHczTHE2b1FiOUxMOVR2T0RheVlFVVgqaTUtbjl6eXljMjV0WEwtT3k1M0czWFZleTVIWGVoaW1sVzM3UyoyU0swZVpwZ2RZRmlZUUpiWnhSbDUwMk9rV2llamVoWWlqR1RzTjRGYTRoTTZMSzJpQ05xck5TOFBjUDdVeFRBQV9f",
		"sdkParams": "eyJ2aWRlb19wYXJhbXMiOnsiZW5jUmVzRW51bSI6MTEyLCJyZXNNb2RlRW51bSI6MSwiY2FwRnBzIjoxNSwiZW5jQlIiOjIzMDAsImVuY0FkanVzdFJlcyI6MCwicW9zUHJlZmVybmVjZUVudW0iOjIsInFvc0NvbnRyb2xNb2RlRW51bSI6MSwiZW5jUmVzRW51bV9zY3JlZW5yZWNvcmQiOjExNCwiY2FwRnBzX3NjcmVlbnJlY29yZCI6MjUsImVuY0JSX3NjcmVlbnJlY29yZCI6MzAwMCwibWljQW5jaG9yXzFfMSI6eyJlbmNSZXNFbnVtIjoxMTIsImNhcEZwcyI6MTUsImVuY0JSIjoxNTAwLCJlbmNCUl9NaW4iOjB9LCJtaWNBdWRpZW5jZV8xXzEiOnsiZW5jUmVzRW51bSI6MTA4LCJjYXBGcHMiOjE1LCJlbmNCUiI6NTAwLCJlbmNCUl9NaW4iOjIwMH0sIm1pY0F1ZGllbmNlXzFfMiI6eyJlbmNSZXNFbnVtIjoxMDgsImNhcEZwcyI6MTUsImVuY0JSIjo1MDAsImVuY0JSX01pbiI6MjAwfSwibWljQWxsX2dyaWRfOSI6eyJlbmNSZXNFbnVtIjo3LCJjYXBGcHMiOjE1LCJlbmNCUiI6NTAwLCJlbmNCUl9NaW4iOjIwMH0sIm1pY19iaWdfOV8xNiI6eyJlbmNSZXNFbnVtIjoxMTQsImNhcEZwcyI6MTUsImVuY0JSIjoyMDAwLCJlbmNCUl9NaW4iOjEwMDB9LCJtaWNfbWlkZGxlXzlfMTYiOnsiZW5jUmVzRW51bSI6MTEyLCJjYXBGcHMiOjE1LCJlbmNCUiI6MTUwMCwiZW5jQlJfTWluIjo3MDB9LCJtaWNfc21hbGxfOV8xNiI6eyJlbmNSZXNFbnVtIjoxMDgsImNhcEZwcyI6MTUsImVuY0JSIjo0MDAsImVuY0JSX01pbiI6MjAwfSwibWljX21pZGRsZV8xXzEiOnsiZW5jUmVzRW51bSI6NywiY2FwRnBzIjoxNSwiZW5jQlIiOjcwMCwiZW5jQlJfTWluIjozNTB9LCJtaWNfc21hbGxfMV8xIjp7ImVuY1Jlc0VudW0iOjcsImNhcEZwcyI6MTUsImVuY0JSIjo0MDAsImVuY0JSX01pbiI6MjAwfSwiYW5jaG9yX3ZpZGVvX2xldmVscyI6W10sInN3aXRjaF9sb2NhbF9zdXBlcl9yZXNvbHV0aW9uIjowfSwiYXVkaW9fcGFyYW1zIjp7ImF1ZGlvUXVhbGl0eSI6M30sImNoYW5uZWxfcGFyYW1zIjp7InVzZXJfZGVmaW5lX3JlY29yZF9pZCI6InZvaXBsaXZlXzFfMjA3ODk2NzQ5Njc3MzEwNTEzNV8xIiwiYXVkaWVuY2VfbW9kZSI6MSwibWljX2FiaWxpdHkiOjEsInFjX2FwcGlkIjoxMzAyOTgzMzQxLCJxY19iaXppZCI6MTExNTgzLCJjZG5fdHJhbnNfaW5mbyI6W3sidXJsIjoiaHR0cDovL3B1bGwtbTEud3hsaXZlY2RuLmNvbS90cnRjXzE0MDA0MTk5MzMvb3JpZ18yMDc4OTY3NDk2NzczMTA1MTM1X25oNDIwLmZsdj9jZG50YWduYW1lPW5oNDIwJmNvbWJ1Zj1tMmZlRVlZYnMyc1F4VEx4RktWbms2UDU3eSUyRiUyRnlZNnpFWUk0SkR2Um5Sc3BWdFF2M3pRTzBoVzJpZkFSdjl5U3NBcE04VmVWTHZDSGFhZkhlSWNiM3VHRWduNFZtdHZnVDFEVkJLek14ZHZJWEFHYUlhTmIlMkZkMVROcDJHU2hzdCUyQkRONG91ckEyOGVxY2p0NnBidHZGYWp4S3MzSEVQWGVLaFVFbUJPZGo3V1ZsUlpONVpBJTNEJmV4cHQ9JmV4dGJ1Zj1DeW43Z1ZHYnAxZDZFR25IbSUyRmZTJTJCdlU1RzZ5azV0RlFRZ3pmbk90cVRldlhnWVBVSFZKJTJGUjFaT2txJTJGaGNqRHRJZGMlMkZyNzdQUCUyQklROWQlMkJYNkNCNGtZRWhtMklGd2xPUWMzeCUyRnNsaTZ1bzBhME53N3RFdXV1ZEh0MWlTMG1vJTJCcVhEcTUzVDcwVG0zMWdZb2JjTzdsZnNIRHdUbTNEb3BDSGhCZ0FoUkVybUFKOEZ3WmFtcmdRRVhGeVpMJTJGNnRmemtYZHlseUd6QzF0cXptUW4xMHJmbzFCUXpsMzhCY0NOJmdpZD0mb3BlbmlkPUEwMzAzOUVDQTJDNDhCNzVBNjY4MzA2NTEwQzkyRjE5JnE9MCZzYz00JnN2PTImdHhTZWNyZXQ9MzljZmNhYTZiYTg4NmRjYmJkZDY4ZWRjNGY3NjI3NjcmdHhUaW1lPTZBNjc2QzVDJnZjb2RlYz0xJnd1PTEmd3hucz0xJnd4dG9rZW49MDRhMTkxNGI1YzJmZWNmNWZkNDIwYTI5YmY0YzI2ZGIiLCJxdWFsaXR5X3RhZyI6MCwidGFnX25hbWUiOiJuaDQyMCIsInJhdGUiOjIwMDAsInZpZGVvX3RhZ190eXBlIjowLCJtZWRpYV90eXBlIjowfSx7InVybCI6Imh0dHA6Ly9wdWxsLW0xLnd4bGl2ZWNkbi5jb20vdHJ0Y18xNDAwNDE5OTMzL29yaWdfMjA3ODk2NzQ5Njc3MzEwNTEzNV9qaDUxNC5mbHY/Y2RudGFnbmFtZT1qaDUxNCZjb21idWY9ME5Kb2lOMThObSUyRiUyRlpLQzZpRiUyRnY2cXdkMGhac0dKem51ajlGNW5ManRxTHh2cFBpQ1Nsb0wxNm5qdTJTOCUyRiUyRkR0WjBDSDlneEg5cjhBRTRaaUppVXhWSjdSQmxTR3RMdm12Q1hVZmZxUjVIcSUyQk5ueGExOCUyQlNlTEFibklJNEhjWVZPQ2R5ZiUyQlRqbzJTc1JvMiUyRndlZVJLRHo1bXdZMGdZdjN3cTJUZCUyRjJKcnpyc2l5bjZYMCUzRCZleHB0PSZleHRidWY9M1JjTUhySUFxMGN6U1RkTDR4RCUyRlpVYXZHZzFxREdoaW5ROVlydjZKMzBwRzN1MkZXYURKajVxZXpBYW5TaFVPZiUyRnE4NXBnYUdaMzc3bzJjMmw5SEN5eiUyQkclMkJIcU1ZNUZNQVFsTTI4V1pBSUQwWVYxZFk1ZnhmemV0TzRMbnFLOFU5bm5mZmJnZmJ1eUdZZiUyRnpoeVdIc0paM0g3NUplZSUyRnFzNDlBMnUzRGNBNEM0Z2wlMkY1QyUyQm9SODBtUno1aGdqZmJmMHRtb1hjVWU0SmUlMkZBUzdZUWptJTJCd3hISmRYSUw5ciZnaWQ9Jm9wZW5pZD1BMDMwMzlFQ0EyQzQ4Qjc1QTY2ODMwNjUxMEM5MkYxOSZxPTMmc2M9NCZzdj0yJnR4U2VjcmV0PTU0ZmVhZmM0NjIyNWYwMzRlOTMwOWRlY2RjY2U3Y2RmJnR4VGltZT02QTY3NkM1QyZ2Y29kZWM9MiZ3dT0xJnd4bnM9MSZ3eHRva2VuPTM2ZjlmOTdkOWZjZTg4NTk1NWZiMjQ2M2I3ZTYxMDE1IiwicXVhbGl0eV90YWciOjEsInRhZ19uYW1lIjoiamg1MTQiLCJyYXRlIjoxNDAwLCJ2aWRlb190YWdfdHlwZSI6MSwidmlkZW9fcXVhbGl0eV9sZXZlbCI6MywidmlkZW9fcXVhbGl0eV9sZXZlbF9kZXNjIjoiSEQiLCJtZWRpYV90eXBlIjowLCJiYWNrX3VybCI6Imh0dHA6Ly9wdWxsLW0xLnd4bGl2ZWNkbi5jb20vdHJ0Y18xNDAwNDE5OTMzL29yaWdfMjA3ODk2NzQ5Njc3MzEwNTEzNV9uaDQyMC5mbHY/Y2RudGFnbmFtZT1uaDQyMCZjb21idWY9YjQyUCUyRndLbG9MT0VrcjRTdk91cHNQTWpWTlpsNXV4NjY2aXJWNiUyRmJNeGZMT1hQamFlZ0FDdlkwR2sxTDV5QkxGVVBJYkIzRVg1Smd3UnFsQm1KWWtSTUlEZ3UzVHNYUlZRWkFXNFBNN05MeDBFV3NJc1NnWVh2M2NFdzVHbzliellmSzZnJTJCNjZRS1l3dHlMdzFYOXRLWGNoTHVMNVYxRGdITmVZRU1oUSUyQlRrRGNmcmVHbyUzRCZleHB0PSZleHRidWY9d2VINVNmbmI4Um5ZVms2cGVMcUFQRFJuQzJlNHF4cEtSckxxdlRnTm91cVd5TExjYkY5aFN3bXJ5YzZZSHNXdUJtcSUyRnVLVHdTVmp2Y0tTU2prR1B1cGk3aW1rN25BQVVhTGhiY244JTJCcjU4RDJzeE1PTGpVZ21BN3dLcmNGUkRkdUlqWUxVNmlReXc0VzRXYzhJWVRVTUUlMkY3eVBlWllKYWdqQWRna2RxNGhHZE5UeHpWbVFMOEpjWmxxMUF0U29CZWdMV2dCaHFvQ3A5ZGlDbGtGMWRXbFdjYmttc2cyc1gmZ2lkPSZvcGVuaWQ9QTAzMDM5RUNBMkM0OEI3NUE2NjgzMDY1MTBDOTJGMTkmcT0wJnNjPTQmc3Y9MiZ0eFNlY3JldD0zOWNmY2FhNmJhODg2ZGNiYmRkNjhlZGM0Zjc2Mjc2NyZ0eFRpbWU9NkE2NzZDNUMmdmNvZGVjPTEmd3U9MSZ3eGV4dD0xXzBfMCZ3eG5zPTEmd3h0b2tlbj0wNGExOTE0YjVjMmZlY2Y1ZmQ0MjBhMjliZjRjMjZkYiJ9LHsicXVhbGl0eV90YWciOjJ9LHsicXVhbGl0eV90YWciOjN9LHsidXJsIjoiaHR0cDovL3B1bGwtbTEud3hsaXZlY2RuLmNvbS90cnRjXzE0MDA0MTk5MzMvb3JpZ18yMDc4OTY3NDk2NzczMTA1MTM1X25oNDIwLmZsdj9jZG50YWduYW1lPW5oNDIwJmNvbWJ1Zj1GQUJnRGhIRkVHc2hmcTkxbWRlekl6aEZtSzhJV0RRJTJCUEZGdUhFMTB4eWtWekRXNGdRTHdXYXd3OWIlMkI4dEk0Q1hTTGt5c2xBZ01xbUdUQSUyRnVEcTBpMnNTbWpFTk1nVCUyRmRUOWNDZjRHaVNkWHUxTVBjdzh2aFVRY0JKQjREbU1SMHF3UWpYWFRqQmcxMGtrNlRlJTJGWktkOXk1OWFHVXpKdHc3UUw0V3dSaFA2ZDJvQyUyRlV5VSUzRCZleHB0PSZleHRidWY9czhiTmV6byUyQmRjemdkV3FnN3NLJTJCMHJvd2F4JTJGJTJGRUlmNDMlMkZGT20xMmJkV1dlejZjcDRjQ1Z3bHhjYjQ3WFRyOGZNSGpTMGt4UnFJNjA4SWVMRndQbGxpamx6QUUyVGx0WHFaZVA5Q0tzNnF4YlV6OGptekVkUHN6Wk4zVjFqQU9DbHhPZGEzRCUyRkRDcEVLMjNibXM5Qm9pZ0k0QUc2RSUyQmJ1Q1pYT3NnVHF2UDducWx2VUcydUglMkZ0ZnN6VlpEbEtSVTdHYTVKNSUyRllxWDVCOVhwbzBsajlwOVZ6WmNwS0NldCUyRiZnaWQ9Jm9wZW5pZD1BMDMwMzlFQ0EyQzQ4Qjc1QTY2ODMwNjUxMEM5MkYxOSZxPTAmc2M9NCZzdj0yJnR4U2VjcmV0PTM5Y2ZjYWE2YmE4ODZkY2JiZGQ2OGVkYzRmNzYyNzY3JnR4VGltZT02QTY3NkM1QyZ2Y29kZWM9MSZ3dT0xJnd4bnM9MSZ3eHRva2VuPTA0YTE5MTRiNWMyZmVjZjVmZDQyMGEyOWJmNGMyNmRiIiwicXVhbGl0eV90YWciOjQsInRhZ19uYW1lIjoibmg0MjAiLCJyYXRlIjoyMDAwLCJ2aWRlb190YWdfdHlwZSI6MCwibWVkaWFfdHlwZSI6MH0seyJ1cmwiOiJodHRwOi8vcHVsbC1tMS53eGxpdmVjZG4uY29tL3RydGNfMTQwMDQxOTkzMy9vcmlnXzIwNzg5Njc0OTY3NzMxMDUxMzVfb3JpZy5mbHY/Y2RudGFnbmFtZT1vcmlnJmNvbWJ1Zj15Z1l6R1ZrcGVDTWY2dmpydzlhcVY0VDRNU2dvQ2k1ZzlyTml5TWlMbGtEV3glMkYyVkJMc2lEY1hwa2VrZmh0am9VV0x2OHJ1R1pvM01zQlVSTDZGJTJGdUFlJTJCNjVBTm55SiUyRlluWThaU3clMkY4bXVINGZYa3NySzJmRlhPbU5tejhJMlU4TTNuNGFEcW13Q2tBYnNseWxkbmUxeHVYM0lnWFRFWWY5JTJGN1JCNkdZeCUyRlQ2QW1nRVVzJTNEJmV4cHQ9JmV4dGJ1Zj1NSGViT0ZBb2d4Y3d6OTFDSWxRb2dCUzZ1Tlh6NERrVmw5OVA3d1IxJTJCOTRYcHNzTDZaWnlHQnJoJTJCR3pWN3lSVjA4JTJCZlV2akw0N0NOalBSTjI0Z0pkeFEyOTZwWFRIQWd5d0ZxTDZ0eEJtTVMybzVUekZHRHFES2tQYWhRYm9pQnNsOXBFTFk1R0xCNTMlMkJ5WjdPRW1Lc25HdlhKM2ZYdkV6bURUZ0MlMkZKb3dKS29Hc3pjZnJySjJpWE5SVDg0aSUyQktzU1BOVWRSMEJsZnA4Z0dMYUR0b3RUMVlWSkVzeVUxVyZnaWQ9Jm9wZW5pZD1BMDMwMzlFQ0EyQzQ4Qjc1QTY2ODMwNjUxMEM5MkYxOSZxPTQmc2M9NCZzdj0yJnR4U2VjcmV0PTdhZGQ1YjY4MDQ2NTg5Mzc5MWY5YmU5NzI5MDc0NGMxJnR4VGltZT02QTY3NkM1QyZ2Y29kZWM9MiZ3dT0xJnd4bnM9MSZ3eHRva2VuPTlmZTY5MTE1YjlkYzIyNzc3M2EyZjIyMTA1ZjJhMDc5IiwicXVhbGl0eV90YWciOjUsInRhZ19uYW1lIjoib3JpZyIsInJhdGUiOjMwMDAsInZpZGVvX3RhZ190eXBlIjoxLCJ2aWRlb19xdWFsaXR5X2xldmVsIjo0LCJ2aWRlb19xdWFsaXR5X2xldmVsX2Rlc2MiOiJTSEQiLCJtZWRpYV90eXBlIjowLCJiYWNrX3VybCI6Imh0dHA6Ly9wdWxsLW0xLnd4bGl2ZWNkbi5jb20vdHJ0Y18xNDAwNDE5OTMzL29yaWdfMjA3ODk2NzQ5Njc3MzEwNTEzNV9uaDQyMC5mbHY/Y2RudGFnbmFtZT1uaDQyMCZjb21idWY9UEhEdFU3d0NidyUyRmZXM3dOcmlQQTMlMkZPQ3NRU29zYklzJTJGSTBxVkx2N2NmeWd1OExWRjZhQUZIOHBiQWhUWFNkTGNFejhSM3piWVFkZndpRnYwblN4SjJRTExBMXFHUG5QSVlQNzlMcDVSOGdHQ0NRYnhJMjI1THFNS01oQjREMnFURmlCaElqU1hQTUpNTEVvR0hGMk8lMkZzVGIlMkZ5U2w3dWJuSkRmbXpvbkNTdWs4OTNNaVZnJTNEJmV4cHQ9JmV4dGJ1Zj1TQ1NtdGFlRExnOWJZMVRhM1BRJTJGaG42aEtpaDBod3NwczF2JTJCT3ZPb3YxN3VMQklodSUyRmVTMHBLa0hnJTJCcVY2alk5VmpGSVZIYzZDcUw2SiUyQllia2FsYldBVFh6S21xNm82WHhMMUlucWwzOURNY0JrS1BSamFZVHJHWXlyQlowS0tKWnduQUxiMGdGZXoxSUxTVnlPT1BxNDRkOVljVXBFNUpaJTJGNEc2MFVibmt0SHBTWGU4cGFQekd4MUFWQ0g4R1JtYUVXNFVsdmVQSnExNmljaTlCZkMlMkJVUEowZURBTHo0JmdpZD0mb3BlbmlkPUEwMzAzOUVDQTJDNDhCNzVBNjY4MzA2NTEwQzkyRjE5JnE9MCZzYz00JnN2PTImdHhTZWNyZXQ9MzljZmNhYTZiYTg4NmRjYmJkZDY4ZWRjNGY3NjI3NjcmdHhUaW1lPTZBNjc2QzVDJnZjb2RlYz0xJnd1PTEmd3hleHQ9MV8wXzAmd3hucz0xJnd4dG9rZW49MDRhMTkxNGI1YzJmZWNmNWZkNDIwYTI5YmY0YzI2ZGIifSx7InVybCI6Imh0dHA6Ly9wdWxsLW0xLnd4bGl2ZWNkbi5jb20vdHJ0Y18xNDAwNDE5OTMzL29yaWdfMjA3ODk2NzQ5Njc3MzEwNTEzNV9uczQwNS5mbHY/Y2RudGFnbmFtZT1uczQwNSZjb21idWY9YzcxNmNyZlcwU3lzMm15ZCUyRjZWYnZYckZHREFLJTJGc0NUZUx0RHZIZ1d0bnZ5ZnFidlFDMlBTeHo2SEd2NU9pdXMwNE1HSnclMkJvWWZ6Y1hDcHJycUlQcTJCMW1JTkpzdlFjRHU0MUFjc2RmQUVyamdRJTJGNmxxNUFYa0hwbXQlMkJLOGxLdjZqcGRVemVDNFdCUmtpTVNUcmZuUTVpa2VtdG84VzVaT1oyZ2VYcmlBeUFpaXUwNSUyRnclM0QmZXhwdD0mZXh0YnVmPSUyQngyWEJpbUpiWkhRdU13TEd6diUyQlNsaWRPT2wwSld0U0djNG81VnpEWFZUeDFDcVBqZDR4QnJ6JTJCa0t6UHRhVDdxSmVGWFRodTRMRSUyQjlrRW8zc1I1V1NWRVg1JTJCV3hYb0J3eTgyRVlSY2lka2FLaFB1QUtUeVdiMW1PQ1F4QWs5S3VuM2ZXSzdjYnlOR3g0NERsMFdqVnVMYlJib3FtbWgwV2NoMEc4bFo0VFlKTUlIZkM3JTJCJTJGMGZCUzJmaGltcDE0JTJCdDNyMiUyRnBRMW02a0U4Z3lPcFclMkJydERRciUyQmR6YWowSCZnaWQ9Jm9wZW5pZD1BMDMwMzlFQ0EyQzQ4Qjc1QTY2ODMwNjUxMEM5MkYxOSZxPTAmc2M9NCZzdj0yJnR4U2VjcmV0PWI5NjcwYTYxZGI2OGMyNTRhYmM4MWIzM2QzOGIzN2EyJnR4VGltZT02QTY3NkM1QyZ2Y29kZWM9MSZ3dT0xJnd4bnM9MSZ3eHRva2VuPTJkMjQ0MDdmOGRmMmNlMTBjNDZlZTg0YjU5MzVmNDMxIiwicXVhbGl0eV90YWciOjYsInRhZ19uYW1lIjoibnM0MDUiLCJyYXRlIjo1MDAsInZpZGVvX3RhZ190eXBlIjowLCJtZWRpYV90eXBlIjowfSx7InVybCI6Imh0dHA6Ly9wdWxsLW0xLnd4bGl2ZWNkbi5jb20vdHJ0Y18xNDAwNDE5OTMzL29yaWdfMjA3ODk2NzQ5Njc3MzEwNTEzNV9qczUwNS5mbHY/Y2RudGFnbmFtZT1qczUwNSZjb21idWY9eSUyQmhQU3l5ZlpySVFhRlQlMkZ1eHB3aDF0ZyUyQlhYZzV6TiUyRlljZ2VRJTJGellERnFxd3MyS2VldktTaVVBT1FrU0szV1J0TXJkMGZpRENBRGxsNk1SWEtMcXQ3eEpVbSUyRkJLM3RpMDdIZ1VwSW1lVzBJR3V3Q0gya1lrQ1ZNNzdLRkJEb3RUM3IlMkZxQWU1Umx5Y2JzJTJCNXA3OEYzRlhNTU9KcFQ3U2x6UkFHSk1FQVVzUGdSWTFCNXFJJTNEJmV4cHQ9JmV4dGJ1Zj1OQlRLbm9hbjUzQjFrdU1NUGNNNnFSY2RJbjlHcFI4NzdGa3dkMkdLSWlNTVRhNW9VWW1rSWQ2SiUyQk5WZkEyNlVqSGFzWHI4eTJwVG9uSUdYRSUyQmx4WmpabWpZcHhuWTNtdEdKdGNjTHYyOEZWd0ZLNDF0QnlpWFVSNjVYJTJGRE1wU0lqeUVzcW5mQ3hvbHRVN0JvT3BQQUJWWENZaW1vcTJKTG9BR1U0RTVSSHY2OHVJYWp5UW9YJTJCY3VFa3Nrbldyc3g0MVNrMWNxM0Y1RXFsbjM1Y20lMkI0bVJBTXd5RWRLY2wmZ2lkPSZvcGVuaWQ9QTAzMDM5RUNBMkM0OEI3NUE2NjgzMDY1MTBDOTJGMTkmcT0xJnNjPTQmc3Y9MiZ0eFNlY3JldD1mZWZjNDZkYzZiMTI2YjJhYjMyZThhYjA4YjQ3NTQxYiZ0eFRpbWU9NkE2NzZDNUMmdmNvZGVjPTImd3U9MSZ3eG5zPTEmd3h0b2tlbj05NjRlMTNlZWJhMjU0NDYwZDNiODJlM2FmZTI1NzExYiIsInF1YWxpdHlfdGFnIjo3LCJ0YWdfbmFtZSI6ImpzNTA1IiwicmF0ZSI6NTAwLCJ2aWRlb190YWdfdHlwZSI6MSwidmlkZW9fcXVhbGl0eV9sZXZlbCI6MSwidmlkZW9fcXVhbGl0eV9sZXZlbF9kZXNjIjoiRmx1ZW50IiwibWVkaWFfdHlwZSI6MCwiYmFja191cmwiOiJodHRwOi8vcHVsbC1tMS53eGxpdmVjZG4uY29tL3RydGNfMTQwMDQxOTkzMy9vcmlnXzIwNzg5Njc0OTY3NzMxMDUxMzVfbmg0MjAuZmx2P2NkbnRhZ25hbWU9bmg0MjAmY29tYnVmPTlkbE5SZ2lPSW5LdW5hNk13aG02RGdjdnVMc0pWUXJVTUxZTUlOb1E0UGRETG9DZEw0JTJGZllyajA1Z3klMkZzRFNoWng3bWQ2SkF5QW9aMG9kYkdCZGJXYm0wQTBmaHRvS2drcEdienZydXMlMkJRcCUyQnNJbDdCS0Nua3JtMG1kNndZNmkxb2VMZVVVNEM2VkJyQTNDY2lUNnhQTXNyV0tmNHZ4WWk2TUZOSSUyRjhzTDZ2Nnl5UVZsOCUzRCZleHB0PSZleHRidWY9cSUyRkEyYXBJcjR3Zk8yaHA0bFhUbGJEOXdKbDZ0eEIlMkJDc3YzYktHUDYxTXBIVlV1N2hEcjclMkJZcFVUZklwSEwwZURtJTJGJTJCR3ZKS1ZVREdPaGNLMXpoaVJXOWtUOGdKa1pUMUo4MjV5MTRmN3AlMkZvaiUyRm80VkNtclEyRFhGeFRXS28yYjY0Y1JZTXN5bGY3cGIlMkI3R2g5b293d1ZtY2RJQ21oUnFLaiUyQk5LcUF2Y0djRFFJRGpndlVnT1dSUjVuaVllWlFDd2cxUEppRUtOZXQ0MU4zQ3oyMWhPWEdiRyUyQlk5R0M1QSZnaWQ9Jm9wZW5pZD1BMDMwMzlFQ0EyQzQ4Qjc1QTY2ODMwNjUxMEM5MkYxOSZxPTAmc2M9NCZzdj0yJnR4U2VjcmV0PTM5Y2ZjYWE2YmE4ODZkY2JiZGQ2OGVkYzRmNzYyNzY3JnR4VGltZT02QTY3NkM1QyZ2Y29kZWM9MSZ3dT0xJnd4ZXh0PTFfMF8wJnd4bnM9MSZ3eHRva2VuPTA0YTE5MTRiNWMyZmVjZjVmZDQyMGEyOWJmNGMyNmRiIn0seyJxdWFsaXR5X3RhZyI6OH1dLCJjZG5faXBzIjpbXSwiY2RuX3F1YWxpdHlfc3ZyY2ZnIjo1LCJjZG5fcXVhbGl0eV9oMjY1YmFja2NmZyI6NCwic3VwcG9ydF9zY3JlZW5fcm90YXRlIjowLCJxb3NfcmVwb3J0X3N3aXRjaCI6MCwicW9zX2NvbnRyb2xfc3dpdGNoIjowLCJzZWlfbW9kZSI6MTA3OCwicGxheWVyX21pbl9jYWNoZV9tcyI6MzAwMCwicGxheWVyX21heF9jYWNoZV9tcyI6MTAwMDAsImdhbWVfYXBwaWQiOiIiLCJobHNfdXJsIjoiaHR0cDovL3B1bGwtbDEud3hsaXZlY2RuLmNvbS90cnRjXzE0MDA0MTk5MzMvb3JpZ18yMDc4OTY3NDk2NzczMTA1MTM1X3d4dHYxMDgwZi5tM3U4P2NkbnRhZ25hbWU9d3h0djEwODBmJmNvbWJ1Zj15QlpvT0Q3bHUlMkIzakZNYWU4NEYzV0M2ZUYxRDBCS002U0lhazB3UU9tMnNVdFBLbDFFem9oQ3A3aDVMSkhiMFdJTjROQjdMZEdDVDNLZVpyQk4lMkZjUGMxakJsdktIT2F3eWE3dkJRUmVsbHhDUmcySUhBaUZ0TkgyNGZVcktnZkNKamRqeFdrZ1RZVWhLQVoySldmMHYxRSUyRiUyQnRPa2taR0xSem5tcU5uTGRrQ0M0SVhDZVFVJTNEJmV4cHQ9JmV4dGJ1Zj1oa0VpR3IlMkJUNzZ2ZkF5NDU5em1sJTJCTXU2TUk5aUVIaDB5cWljUU81RW40dlhXdDRlV1RzVEljeTdxZW9nJTJGSndoYkhHRWRHalglMkJmZVVDRW5vYnc4WlhCWDFXeVo1RTFVJTJCeGRXSXJLYmpWNUNoUEw1ZzFHaFA3bFlvQ3hGMHdSRXUlMkZFQWdrYXkyemlTQkdyYWJxZXh0eWF1SnduVlJCOW1VN2p5MlBaUjZSMXpEUExYajclMkZjM29pS216RTUwRE04UEoyTFRsdlBBRmRFQk1HOGltTTNzWEtlUXM0c0p3a0FCJmdpZD0mb3BlbmlkPUEwMzAzOUVDQTJDNDhCNzVBNjY4MzA2NTEwQzkyRjE5JnE9MCZzYz0zMiZzdj0yJnR4U2VjcmV0PTBkOWQ5NjY2ZDExZjc5MzU1NTkwMTA3N2FkZDI5MDA3JnR4VGltZT02QTY3NkM1QyZ2Y29kZWM9MSZ3dT0xJnd4bnM9MSZ3eHRva2VuPWY2YjFmYzVmMWMyMTgwYWUzOTBiZDZmZTUzMTA0MTA3IiwicDJwX3N3aXRjaCI6MCwicDJwX3VwbG9hZF9zd2l0Y2giOjAsInAycF9zdGF0X3N3aXRjaCI6MCwicDJwX2RlYnVnX2xvZ19zd2l0Y2giOjAsInAycF9tYXhfbG9hZCI6MCwicDJwX21heF9idWZmZXJfc2l6ZSI6MCwicDJwX3VwbG9hZF9kYXdhbmdrYV9zd2l0Y2giOjAsInAycF9hcHBpZCI6Ik5qQTJOVE0zTWpRd05qSTFOVFpsTWpGaVpXWmtZMkV6IiwicDJwX2tleSI6ImQxaGtiVXhXVlROUFpqVXdlRVpuVHc9PSIsInAycF9zZWNyZXQiOiJRMUJMU0dOWFRVYzJWa0ZyTlVaNVNVZHJibTAwVW5keE0xRlBSVmRET0d3PSIsInAycF92ZXJpZnlfc3RyZWFtIjowLCJxb3NfcmVwb3J0X2ludGVydmFsX3NlY29uZHMiOjUsImJha19kb21haW4iOlsidm9pcGZpbmRlcmxpdmVwbGF5MS53eHFjbG91ZC5xcS5jb20iXSwiY3VzdG9tX3JlbmRlcl9wYXJhbSI6MjAsIndhdmVqYm1fZmxhZyI6MCwid2F2ZWpibV9tb2RlIjoxLCJ3YXZlamJtX21pbl9zcGVlZF9yYXRlIjo3MCwid2F2ZWpibV9tYXhfc3BlZWRfcmF0ZSI6MTMwLCJsZWJfbWluX2NhY2hlX21zIjo4MDAsImxlYl9tYXhfY2FjaGVfbXMiOjI1MDAsInN2cl9zd2l0Y2hfbGlzdCI6WzFdLCJkZXZpY2VfbGV2ZWwiOjAsImNwdV9zY29yZSI6MCwiZ3B1X3Njb3JlIjowLCJmbHZfZ29wX2NhY2hlIjowLCJmbHZfaXB2Nl9maXJzdCI6MCwicDJwX2RvbWFpbiI6IiIsInAycF91cGRhdGVfdmlkZW9fY2FjaGVfc3dpdGNoIjowLCJ0cnRjX2FpX2JvdF91c3JfaW5mbyI6eyJzZGtfdXNlcl9pZCI6Im85aEhuNVNXRmhYUXpUTFJkN2RUQk9sNWhDdHNfYWlib3QifX0sImNoYW5uZWxfcGFyYW1zX2Rlc2MiOltdfQ==",
		"sdkCreateUserId": "o9hHn5SWFhXQzTLRd7dTBOl5hCts",
		"expireForSig": "15552000",
		"expireForPmk": "86400",
		"liveId": "2078967496773105135",
		"liveCdnUrl": "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_2078967496773105135_orig.flv?cdntagname=orig&combuf=ygYzGVkpeCMf6vjrw9aqV4T4MSgoCi5g9rNiyMiLlkDWx%2F2VBLsiDcXpkekfhtjoUWLv8ruGZo3MsBURL6F%2FuAe%2B65ANnyJ%2FYnY8ZSw%2F8muH4fXksrK2fFXOmNmz8I2U8M3n4aDqmwCkAbslyldne1xuX3IgXTEYf9%2F7RB6GYx%2FT6AmgEUs%3D&expt=&extbuf=MHebOFAogxcwz91CIlQogBS6uNXz4DkVl99P7wR1%2B94XpssL6ZZyGBrh%2BGzV7yRV08%2BfUvjL47CNjPRN24gJdxQ296pXTHAgywFqL6txBmMS2o5TzFGDqDKkPahQboiBsl9pELY5GLB53%2ByZ7OEmKsnGvXJ3fXvEzmDTgC%2FJowJKoGszcfrrJ2iXNRT84i%2BKsSPNUdR0Blfp8gGLaDtotT1YVJEsyU1W&gid=&openid=A03039ECA2C48B75A668306510C92F19&q=4&sc=4&sv=2&txSecret=7add5b680465893791f9be97290744c1&txTime=6A676C5C&vcodec=2&wu=1&wxns=1&wxtoken=9fe69115b9dc227773a2f22105f2a079"
	},
	"liveInfo": {
	"liveId": "2078967496773105135",
	"liveStatus": 1,
	"streamUrl": "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_2078967496773105135_orig.flv?cdntagname=orig&combuf=ygYzGVkpeCMf6vjrw9aqV4T4MSgoCi5g9rNiyMiLlkDWx%2F2VBLsiDcXpkekfhtjoUWLv8ruGZo3MsBURL6F%2FuAe%2B65ANnyJ%2FYnY8ZSw%2F8muH4fXksrK2fFXOmNmz8I2U8M3n4aDqmwCkAbslyldne1xuX3IgXTEYf9%2F7RB6GYx%2FT6AmgEUs%3D&expt=&extbuf=MHebOFAogxcwz91CIlQogBS6uNXz4DkVl99P7wR1%2B94XpssL6ZZyGBrh%2BGzV7yRV08%2BfUvjL47CNjPRN24gJdxQ296pXTHAgywFqL6txBmMS2o5TzFGDqDKkPahQboiBsl9pELY5GLB53%2ByZ7OEmKsnGvXJ3fXvEzmDTgC%2FJowJKoGszcfrrJ2iXNRT84i%2BKsSPNUdR0Blfp8gGLaDtotT1YVJEsyU1W&gid=&openid=A03039ECA2C48B75A668306510C92F19&q=4&sc=4&sv=2&txSecret=7add5b680465893791f9be97290744c1&txTime=6A676C5C&vcodec=2&wu=1&wxns=1&wxtoken=9fe69115b9dc227773a2f22105f2a079",
	"startTime": 1785075244,
	"likeCnt": 1209,
	"endTime": 0,
	"liveExtInfo": {
	"anchorStatusBuffer": "eyJ0aW1lX21zIjoiMTc4NTA3NjA2ODIzOSIsInN0YXR1c19mbGFnIjoiNTkwODQ3MjAwMCIsImdhbWVfam9pbl90ZWFtX3NldHRpbmciOnsiY3VyX2pvaW5fdGVhbV9tb2RlIjowLCJwYXltZW50X3NldHRpbmciOnsic2V0dGVkX3BheW1lbnQiOjB9fSwibWljX3NldHRpbmciOnsic2V0dGluZ19mbGFnIjowLCJoaWdobGlnaHRfbWljX3BlcnNvbiI6ZmFsc2UsInBrX3NldHRpbmdfZmxhZyI6MCwibWljX2xheW91dF9iYXNlX21vZGUiOjF9LCJsb3R0ZXJ5X3NldHRpbmciOnsic2V0dGluZ19mbGFnIjoxLCJhdHRlbmRfdHlwZSI6Mn0sInZlcnNpb24iOjIsInRzX21hcCI6eyIyNSI6MTc4NTA3NjA2ODIzOX0sImxpdmVfcmVwbGF5X3NldHRpbmciOnsiYXV0b19nZW5fbGl2ZV9yZXBsYXkiOmZhbHNlLCJpbnRlbGxpZ2VudGx5X2dlbl9yZXBsYXlfaGlnaGxpZ2h0IjpmYWxzZSwiZW5hYmxlX3JlcGxheV9kdW1wX2Rhbm11IjpmYWxzZSwiY2FuX3VzZV9pbnRlbGxpZ2VudGx5X2dlbl9yZXBsYXlfaGlnaGxpZ2h0Ijp0cnVlfX0="
	},
	"liveSdkChannelInfo": {
	"audienceMode": 1,
	"enableP2p": 0
	},
	"participantCount": 3328,
	"rewardTotalAmountInWecoin": "2016",
	"sourceType": 0,
	"rewardTotalAmountInHeat": "2016",
	"newLikeCnt": "1209",
	"heatValue": "2213",
	"layerShowInfo": {
	"showType": 0,
	"accumulatedSeconds": 0
	},
	"purchaseInfo": {
	"chargeFlag": 0,
	"isPurchased": true
	},
	"anchorStatusFlag": "5908472000",
	"liveActivityType": [],
	"liveFlag": 655525,
	"multiReason": [],
	"coverInfo": {
	"effectType": 0
	},
	"liveConcertInfo": {
	"isConcertLive": false,
	"activityId": "",
	"hasTicket": 0
	},
	"liveHeatValue": "2022",
	"entranceAdInfos": [],
	"memberInfo": {
	"canJoinMember": false,
	"memberSetting": []
	},
	"memberRelationshipInfo": {
	"memberLevel": 0
	},
	"themeColorInfo": {
	"infos": [],
	"version": "6"
	},
	"livePrivilegeInfo": {
	"audienceNoPrivilege": 0
	},
	"screenOrientationInfo": {
	"screenOrientation": 0
	},
	"liveHeatValueStr": "",
	"subSourceType": 0,
	"forwardCnt": "19"
	},
	"liveMicInfo": {
	"micType": 3,
	"micAudienceList": [],
	"battleInfo": {
	"battleId": "2078967496773105135_4A29016D8640D882742773EBA5FF6213_3A11BD29-4354-49CA-81BF-685F5EE22518",
	"battleSeq": "0",
	"status": 10,
	"timeLeft": 24,
	"playerInfo": [
		{
		"finderUsername": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
		"rewardWecoin": "2000",
		"isAccepted": true,
		"critQuestInfo": {
		"startTime": "1785076230",
		"endTime": "1785076324",
		"questDuration": 90,
		"timeIntervalBeforeQuest": 80,
		"progress": [
		{
			"targetType": 2,
			"targetNum": "500",
			"currentNum": "2000",
			"giftProductId": "",
			"giftInfo": {
			"rewardProductId": "",
			"thumbnailFileUrl": "",
			"previewPagUrl": "",
			"animationPagUrl": "",
			"thumbnailFileMd5": "",
			"previewPagMd5": "",
			"animationPagMd5": "",
			"name": "",
			"price": 0,
			"giftType": 0,
			"unlockIntimacyLevel": 0,
			"flag": 0,
			"landscapeAnimationPagUrl": "",
			"landscapeAnimationPagMd5": "",
			"customInfo": "",
			"disableCombo": false,
			"multiAnimationList": [],
			"useRfxPag": false,
			"pagConfig": {}
			}
		}
		],
		"reward": [
		{
			"pkExtraTimesMulti100": 200,
			"buffDuration": 70,
			"startTime": "1785076330",
			"endTime": "1785076394"
		}
		],
		"deliveryCritQuest": true,
		"isCritQuestAccomplished": true,
		"currentStage": 5,
		"critQuestId": "crit_quest_2078967496773105135_4A29016D8640D882742773EBA5FF6213_3A11BD29-4354-49CA-81BF-685F5EE22518_1785076221",
		"isQuestEnd": true
		},
		"extraRewardWecoin": "0",
		"isApplicant": true,
		"count": "2000",
		"boardKey": "AAQ1niAFAAABAAAAAABt0rFNb8qFDiJ+qBlmaiAAAADZUsq2irJfd3YV8nBJDCZdtai/e9xnkOJ88izhORWhH8eoVX7OS3rDrv5ysPn3Ucqq8daY2TDDvGoGuXXBbUfro3bL6eXOo3K3SMnMFc4Nuw85FhISrdYGFeX8rsxdlh4YEE7KQ9jLONIO0oF6btEwkHABQqndmN51QnxY9F1ESH4t2fWUc1O8SabwK8bBmNUBhdnu0WPzRFwvzgLQumDlVwslFT2VSlVfJjFGsQmhWt8gd1I0dSoJuBBepMtpq2NR+jp1oMD0lZo=",
		"boardType": 2,
		"sdkUserId": "o9hHn5SWFhXQzTLRd7dTBOl5hCts"
		},
		{
		"finderUsername": "v2_060000231003b20faec8c6eb881fc4d7ca0de537b077c863942ef6839ea434138e32e2851773@finder",
		"rewardWecoin": "27",
		"isAccepted": true,
		"extraRewardWecoin": "0",
		"isApplicant": false,
		"count": "27",
		"boardKey": "AAQ1niAFAAABAAAAAABVM2KKcRHqhEk5qBlmaiAAAADZUsq2irJfd3YV8nBJDCZdtai/e9xnkOJ88izhORWhH7JLm5qO+BSCFlJ/M4uN2vGl66qkGfYGXaZ5VkwRYcJ2gfCeIWo/rPheJvTgDz9OTI1Eea8zy7M8gYMWUZrKpyiAODHm1lKIPnPvm3VD6Pn1wW7D1sITaeUw0TdShrCqBCzhMpCPKrueOur2RWxdNmFPQs+tUsmf8VA5Ztfk2knxP1/B8HEoTmZuqFRhL3he6uZDrcZjTNKIs8PH6dqPKNOqUASLPHqPgJU=",
		"boardType": 2,
		"sdkUserId": "o9hHn5ZGkriXMdYU0lFlS0c8a6uE"
		},
		{
		"finderUsername": "v2_060000231003b20faec8c7e38e10cbd1ca0ce533b07750cc371d4308ea2faad77065dba095f9@finder",
		"rewardWecoin": "523",
		"isAccepted": true,
		"extraRewardWecoin": "0",
		"isApplicant": false,
		"count": "523",
		"boardKey": "AAQ1niAFAAABAAAAAADkuFUH2UATCnkjqBlmaiAAAADZUsq2irJfd3YV8nBJDCZdtai/e9xnkOJ88izhORWhH+1iw9OOCMaYWTfAa+RBlnRSM82gruhn46oNT2RKMcEfBoB//gU154K5KZrVDa5TG0L2IpFg9no57yQi0ctzcQYKIiUKLMYWdrSzXxZnuZ8+Kxy/8I9ydYt8LMpyCCw4MOwSeja6cOSXEkZcMhJIsQUTtiVh1ZN/GmHc/aE171z+7MsEIrGHvY9q8Wg0oADTMZ8s1QS5inoF6WBBBRWiaeHNH0TWBQFiVcE=",
		"boardType": 2,
		"sdkUserId": "o9hHn5YrjHzIQsJqDLj0izPw7Z-k"
		},
		{
		"finderUsername": "v2_060000231003b20faec8c7e2811ecad5cc07eb34b0777409b80102915d29d412a2f8236b636d@finder",
		"rewardWecoin": "2684",
		"isAccepted": true,
		"extraRewardWecoin": "1155",
		"isApplicant": false,
		"count": "2684",
		"boardKey": "AAQ1niAFAAABAAAAAAAGkqAwA2GUcKlcqBlmaiAAAADZUsq2irJfd3YV8nBJDCZdtai/e9xnkOJ88izhORWhH/lPQsZOnaMZMTCty9mrXTqS0chpK/3FVuT8DLVaapKBZ4EOpDT9XTd4gpCN6JiJh3cTmGPaMFJZhC82KtFs0/LqIILG9ZNx5YZsZaEtezLjReZ50UqdUJcHG40UTX4IZk6GfuPdWe4nvkwDt/fM0tnwv8A0V+dEVgIy0q3zBPGCtK50cKYHYN9jYpiMNTPGXGhv8MuwWpKcFkPbNRR2f7X264xK9CBbiIc=",
		"boardType": 2,
		"sdkUserId": "o9hHn5WEaZV9LAYN_e3imWnf4-Gk"
		}
	],
	"battleType": 0,
	"battleMode": 1,
	"battleTeams": [
		{
		"members": [
		{
		"finderUsername": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
		"sdkUserId": "o9hHn5SWFhXQzTLRd7dTBOl5hCts"
		}
		],
		"rewardWecoin": "2000",
		"count": "2000",
		"teamId": 1
		},
		{
		"members": [
		{
		"finderUsername": "v2_060000231003b20faec8c7e38e10cbd1ca0ce533b07750cc371d4308ea2faad77065dba095f9@finder",
		"sdkUserId": "o9hHn5YrjHzIQsJqDLj0izPw7Z-k"
		}
		],
		"rewardWecoin": "523",
		"count": "523",
		"teamId": 2
		},
		{
		"members": [
		{
		"finderUsername": "v2_060000231003b20faec8c7e2811ecad5cc07eb34b0777409b80102915d29d412a2f8236b636d@finder",
		"sdkUserId": "o9hHn5WEaZV9LAYN_e3imWnf4-Gk"
		}
		],
		"rewardWecoin": "2684",
		"count": "2684",
		"teamId": 3
		},
		{
		"members": [
		{
		"finderUsername": "v2_060000231003b20faec8c6eb881fc4d7ca0de537b077c863942ef6839ea434138e32e2851773@finder",
		"sdkUserId": "o9hHn5ZGkriXMdYU0lFlS0c8a6uE"
		}
		],
		"rewardWecoin": "27",
		"count": "27",
		"teamId": 4
		}
	],
	"indicatorType": 0,
	"indicatorParameter": "",
	"extraInfo": {
		"currentExtraTimesMulti100": 100,
		"extraItems": [
		{
		"totalExtraCount": "0",
		"type": 1
		}
		]
	},
	"battleScope": 0,
	"isDisableNextBattle": false,
	"battleDuration": 300,
	"dataTimestampMs": "1785076430753",
	"battleLayout": 0,
	"lastRewardRecordCreateTime": "1785076412"
	},
	"enableCrossLiveRoomMic": true,
	"wording": {
	"adaptivePatternWording": "根据接入人数，展示连麦人画面",
	"seatPatternWording": "开通8个固定麦位，观众可选位申请",
	"leaderPatternWording": "主要突出一名连麦者",
	"soloBattleModeWording": "连麦主播分别累计PK值，最多4人",
	"teamupBattleModeWording": "2队PK，每队至少1人",
	"battleIndicatorRewardHeatWording": "热度最高即可获胜",
	"battleIndicatorSpecificGiftNumWording": "指定礼物获得数量越多即可获胜",
	"audienceSoloBattlePatternWording": "所有观众分别累计PK值，最多8名观众",
	"audienceTeamupBattlePatternWording": "将观众分为2队PK，主播不参与比拼",
	"acrossRoomBattleWithAudiencePatternWording": "带观众与连麦主播PK，最多带4名观众"
	},
	"newPkMicInfos": [
	{
		"liveMicId": "KKeFHvevW/3XKVGnR16gBA==",
		"micSdkUserId": "o9hHn5ZGkriXMdYU0lFlS0c8a6uE",
		"micContact": {
		"contact": {
		"username": "v2_060000231003b20faec8c6eb881fc4d7ca0de537b077c863942ef6839ea434138e32e2851773@finder",
		"nickname": "木冉777.",
		"headUrl": "https://wx.qlogo.cn/finderhead/bnOqpXtQ5Q6AfDAiabFQ2PdtpnI71hQiaaicCg1MsqkfMnEvPawiaGRt3AhvcHMB2lDicXvb48hbYoLM/0",
		"signature": "遇见的都是天意\n美术老师🎨\n擅长现代舞，古典舞",
		"authInfo": {
		"authIconType": 0,
		"authIconUrl": ""
		},
		"extInfo": {
		"country": "CN",
		"province": "Guangdong",
		"city": "Guangzhou",
		"sex": 2
		},
		"liveStatus": 1,
		"liveInfo": {
		"liveCoverImgs": [],
		"objectId": "14974182702453295482"
		},
		"bindInfo": [],
		"menu": [],
		"referenceInfo": []
		},
		"displayNickname": "木冉777.",
		"liveIdentity": 3,
		"liveBgImgUrl": "http://wxapp.tc.qq.com/292/20304/stodownload?filekey=30350201010421301f020201240402535a0410e5606af789cab759c1bc8b34207d41fc02030f93ce040d00000004627466730000000132&storeid=269e247a300011fdd694f9c840000012400004f50535a2af02b61573533e99&hy=SZ&m=e5606af789cab759c1bc8b34207d41fc",
		"liveContactExtInfo": "CAA=",
		"badgeInfos": [],
		"voiceLiveImg": {
		"voiceLiveImg": "http://wxapp.tc.qq.com/292/20304/stodownload?filekey=30350201010421301f020201240402535a0410e5606af789cab759c1bc8b34207d41fc02030f93ce040d00000004627466730000000132&storeid=269e247a300011fdd694f9c840000012400004f50535a2af02b61573533e99&hy=SZ&m=e5606af789cab759c1bc8b34207d41fc"
		}
		},
		"status": 2,
		"micAudienceList": [],
		"micSdkLiveId": 91509655,
		"extFlag": "0"
	},
	{
		"liveMicId": "sKSgndlbOipRRrO5mBV/yQ==",
		"micSdkUserId": "o9hHn5YrjHzIQsJqDLj0izPw7Z-k",
		"micContact": {
		"contact": {
		"username": "v2_060000231003b20faec8c7e38e10cbd1ca0ce533b07750cc371d4308ea2faad77065dba095f9@finder",
		"nickname": "メ雨晨ゞヤ2598",
		"headUrl": "https://wx.qlogo.cn/finderhead/ibhzWy4ibIEpBUia3X7MMVibFIDiax3bAsPnbBVHB6Z3Ig956w5OdEcTdmSSMafXoFqGo2HRoFWfGricc/0",
		"signature": "人生短暂，珍惜当下，感谢那么好看的你，还关注了我。本人已经实名认证。温馨提示:其他小号都是别人搬运为了吸粉，大家注意识别，不要被骗。",
		"authInfo": {
		"authIconType": 1,
		"authProfession": "娱乐主播",
		"authIconUrl": "https://dldir1v6.qq.com/weixin/checkresupdate/auth_icon_level3_2e2f94615c1e4651a25a7e0446f63135.png"
		},
		"extInfo": {
		"country": "CN",
		"province": "Jiangsu",
		"city": "Nanjing",
		"sex": 2
		},
		"liveStatus": 1,
		"liveInfo": {
		"liveCoverImgs": [],
		"objectId": "14974238380652237226"
		},
		"bindInfo": [],
		"menu": [],
		"referenceInfo": []
		},
		"displayNickname": "メ雨晨ゞヤ2598",
		"liveIdentity": 3,
		"liveBgImgUrl": "http://wxapp.tc.qq.com/292/20350/stodownload?filekey=30340201010420301e020201240402534804100a304e5f5799e9c1b8dd663398ea61250202689d040d00000004627466730000000132&storeid=26a4dcb3d0001e57349772fb20000012400004f7e534824783bc1e7b3a1d9c&m=0a304e5f5799e9c1b8dd663398ea6125&hy=SH",
		"liveContactExtInfo": "CAA=",
		"badgeInfos": [],
		"voiceLiveImg": {
		"voiceLiveImg": "http://wxapp.tc.qq.com/292/20350/stodownload?filekey=30340201010420301e020201240402534804100a304e5f5799e9c1b8dd663398ea61250202689d040d00000004627466730000000132&storeid=26a4dcb3d0001e57349772fb20000012400004f7e534824783bc1e7b3a1d9c&m=0a304e5f5799e9c1b8dd663398ea6125&hy=SH"
		}
		},
		"status": 2,
		"micAudienceList": [],
		"micSdkLiveId": 160708769,
		"extFlag": "1"
	},
	{
		"liveMicId": "Y8GMs6ky4F+Rleh6NFfXUA==",
		"micSdkUserId": "o9hHn5WEaZV9LAYN_e3imWnf4-Gk",
		"micContact": {
		"contact": {
		"username": "v2_060000231003b20faec8c7e2811ecad5cc07eb34b0777409b80102915d29d412a2f8236b636d@finder",
		"nickname": "向小扣",
		"headUrl": "https://wx.qlogo.cn/finderhead/5QIsgUs6FK8w60eTfXiaX2iaTf2ulpNElDIicbdl9RoSTe9ao7lUmTT1UFnribpjZAAZuuBbqwMu7ts/0",
		"signature": " 希望点进我的主页你能开心 \n谢谢大家关注💤",
		"authInfo": {
		"authIconType": 1,
		"authProfession": "生活博主",
		"authIconUrl": "https://dldir1v6.qq.com/weixin/checkresupdate/auth_icon_level3_2e2f94615c1e4651a25a7e0446f63135.png"
		},
		"extInfo": {
		"country": "CN",
		"province": "Guangdong",
		"city": "Foshan",
		"sex": 2
		},
		"liveStatus": 1,
		"liveInfo": {
		"liveCoverImgs": [],
		"objectId": "14974252374602222049"
		},
		"bindInfo": [],
		"menu": [],
		"referenceInfo": []
		},
		"displayNickname": "向小扣",
		"liveIdentity": 3,
		"liveBgImgUrl": "http://wxapp.tc.qq.com/292/20350/stodownload?filekey=30350201010421301f020201240402535a04105adf1ee7c5326d1862353f36541d2ac80203018676040d00000004627466730000000132&storeid=2687b88d4000f2b5739f511f30000012400004f7e535a198931c156c8148e7&hy=SZ&m=5adf1ee7c5326d1862353f36541d2ac8",
		"liveContactExtInfo": "CAA=",
		"badgeInfos": [],
		"voiceLiveImg": {
		"voiceLiveImg": "http://wxapp.tc.qq.com/292/20350/stodownload?filekey=30350201010421301f020201240402535a04105adf1ee7c5326d1862353f36541d2ac80203018676040d00000004627466730000000132&storeid=2687b88d4000f2b5739f511f30000012400004f7e535a198931c156c8148e7&hy=SZ&m=5adf1ee7c5326d1862353f36541d2ac8"
		}
		},
		"status": 2,
		"micAudienceList": [],
		"micSdkLiveId": 88346643,
		"extFlag": "1"
	}
	],
	"anchorNewPkInfo": {
	"sessionId": "rYzYkXgzYaj/QEHCVZ9Kzw==",
	"vroomId": "pkvroom_13104804816724993_1785076104",
	"vroomIdVersion": "2"
	},
	"battleSettingInfo": {
	"battleDuration": [
		180,
		300,
		600,
		900
	],
	"defaultBattleDuration": 300,
	"multipleTeamsBackgroundPicUrl": "http://dldir1.qq.com/weixin/ios/solo_e6adca5ba72049ae8b1123bf76282618.png",
	"twoTeamsBackgroundPicUrl": "http://dldir1.qq.com/weixin/ios/teamup_8a9f58ff8fd64d82b89b1bf0a9892359.png"
	},
	"micConfig": {
	"modeConfigList": [],
	"displayIntervalS": 30,
	"micTopicTitle": "话题"
	},
	"newPkMicInfosForBoard": []
	},
	"userInfo": {
	"enableComment": 1,
	"enableFriendChat": 1
	},
	"selfContact": {
	"enableComment": 1,
	"liveContactFlag": 11,
	"badgeInfos": [
	{
		"badgeType": 2,
		"badgeLevel": 0
	}
	]
	},
	"tips": {
	"lawTips": "Welcome to Channels Live! Weixin prohibits live streaming by minors, solicitation of viewers for private transactions by hosts, and any illegal activities, including spreading lewd content, engaging in fraud, smoking or alcohol abuse. Be cautious to avoid property and personal loss. Report to us if you find any violations."
	},
	"templateInfoList": [],
	"aliasInfo": [
	{
	"nickname": "李涛",
	"headImgUrl": "http://wx.qlogo.cn/mmhead/ZMdxSDafpxQD4xIyibhpnxavmhg7uibyrwL2UhROmjgqgM8phDWZnyPg/0",
	"roleType": 1
	},
	{
	"nickname": "测试0906",
	"headImgUrl": "https://wx.qlogo.cn/finderhead/ZMdxSDafpxQatchbcRZdYpGznsRYesTlZxQHp8Y5CNhl2RPSJjtqSibqDla8Rlo6FeBMOq2tQ7Ko/0",
	"roleType": 3
	}
	],
	"currentAliasRoleType": 1,
	"nextAliasModAvailableTime": "0",
	"verifyInfoBuf": "CmCFcmQmKfwvEPOIJYjuiTU/tx6bGOA36m6Bh+BaJcVC+kyfPpBZxJaDuyYL3O4ZHL1lK7+g4O+DKL1pgr7tSSe9I/KOX7R6KEQuQuOYd1NzXfM985KWNWb7p11r8lZlP/M=",
	"redpacketCliBuff": "CLO0lKns6NjnzwEQ7-v5xca9_uwcIAAoATCXlsn2hNijFzoTNzUxMDAzOTU0Njk3NjMzNzkwMUAAUgA",
	"redpacketReferChatroomIdList": [],
	"ecSource": "findershopbillingctx__0_o9hHn5TBuFNJvai5p-_5eOWlbabk_1785076444771_1998339591",
	"cheerInfo": {
	"cheerIconInfo": [
	{
		"iconUrl": "http://wxapp.tc.qq.com/251/20304/stodownload?m=1a3e05118707fdfa6391469ed72db56c&filekey=30340201010420301e020200fb0402535a04101a3e05118707fdfa6391469ed72db56c020268cb040d00000004627466730000000132&hy=SZ&storeid=563bc1562000e65280004717a000000fb00004f50535a13da8970b66cb82e3&dotrans=0&bizid=1023&adaptivelytrans=0",
		"sampRate": 17
	},
	{
		"iconUrl": "https://wxapp.tc.qq.com/251/20304/stodownload?m=08c2a52736311395fb8e3906b7b89191&filekey=30450201010420301e020200fb04025348041008c2a52736311395fb8e3906b7b8919102024101041e000000046274667300000001310000000861707073746f72650000000131&hy=SH&storeid=32303231303932343230353833303030306131336364303030303030303064326536346630393030303030306662&dotrans=0&bizid=1023&adaptivelytrans=0",
		"sampRate": 17
	},
	{
		"iconUrl": "https://wxapp.tc.qq.com/251/20304/stodownload?m=9011c5b199e1fd1521cd7bb8249a2985&filekey=30450201010420301e020200fb0402534804109011c5b199e1fd1521cd7bb8249a29850202624b041e000000046274667300000001310000000861707073746f72650000000131&hy=SH&storeid=32303231303932343230353833303030306134306338303030303030303034653032396430393030303030306662&dotrans=0&bizid=1023&adaptivelytrans=0",
		"sampRate": 16
	},
	{
		"iconUrl": "https://wxapp.tc.qq.com/251/20304/stodownload?m=b74a1f7abe9979f0dee4135e3fb48666&filekey=30450201010420301e020200fb040253480410b74a1f7abe9979f0dee4135e3fb48666020242aa041e000000046274667300000001310000000861707073746f72650000000131&hy=SH&storeid=32303231303932343230353833303030306133653231303030303030303064653033396430393030303030306662&dotrans=0&bizid=1023&adaptivelytrans=0",
		"sampRate": 17
	},
	{
		"iconUrl": "https://wxapp.tc.qq.com/251/20304/stodownload?m=c16e28cd8e54a0b6bc1a9ed45bdb5d02&filekey=30450201010420301e020200fb040253480410c16e28cd8e54a0b6bc1a9ed45bdb5d0202025ea7041e000000046274667300000001310000000861707073746f72650000000131&hy=SH&storeid=32303231303932343230353833303030306131633733303030303030303037313737343530393030303030306662&dotrans=0&bizid=1023&adaptivelytrans=0",
		"sampRate": 17
	},
	{
		"iconUrl": "https://wxapp.tc.qq.com/251/20304/stodownload?m=edd7521d1169e3fb3a155bdf46e5a60f&filekey=30450201010420301e020200fb040253480410edd7521d1169e3fb3a155bdf46e5a60f02026483041e000000046274667300000001310000000861707073746f72650000000131&hy=SH&storeid=32303231303932343230353833303030306134336233303030303030303064346331363130393030303030306662&dotrans=0&bizid=1023&adaptivelytrans=0",
		"sampRate": 16
	}
	],
	"enable": 1,
	"animation": [],
	"cheerInfoId": "finderactivity_default_cheer_info_id"
	},
	"enableCheerSpecialEffect": 1,
	"disableExtraSyncCmds": [
	13,
	39,
	44,
	15,
	41,
	16,
	29,
	17,
	14,
	43,
	20,
	21,
	22,
	23,
	27,
	31,
	5,
	6,
	11
	],
	"enableExtraSyncCmds": [
	1,
	2,
	3,
	4,
	18,
	7,
	8,
	10,
	25,
	26
	],
	"fanClubInfo": {
	"clubName": "吃饱饱",
	"clubCreated": true,
	"memberCount": 4551,
	"detailPageUrl": "https://channels-aladin.wxqcloud.qq.com/aladin/html/82e87e64-d6c6-4361-b48c-7b8fd6d9cd14.html?rd=wechat_redirect&hexBackgroundColor=191919#/",
	"enableFanClub": 1,
	"defaultIntimacy": 2,
	"intimacyRefreshInterval": 10,
	"enableBulletin": true,
	"isSuperFanClub": false,
	"superFansCount": 5
	},
	"isFanClubMember": false,
	"liveDescription": "谁可以无缘无故给我刷个岛",
	"isAssistantRole": false,
	"newPromoteInfo": {},
	"anchorSwitchFlag": 15987199,
	"liveFunctionSwitchFlags": "16",
	"anchorLiveExtFlag": "7577",
	"liveRoomImg": {
	"voiceLiveImg": "http://wxapp.tc.qq.com/292/20350/stodownload?filekey=30350201010421301f02020124040253480410faa4e12f5781dbcea13de6912570bcb8020301355a040d00000004627466730000000132&storeid=268cd44fa0009a65a20f11d7e0000012400004f7e5348290321f1579137cca&hy=SH&m=faa4e12f5781dbcea13de6912570bcb8",
	"imgType": 0
	},
	"liveExtFlag": 268446406,
	"songListInfo": {
	"singingSongName": "",
	"hasLiveSongList": false,
	"enableCreateLiveSongs": true,
	"isNewVersionSongList": false
	},
	"resourcePreloadInfo": {
	"preloadFlag": "35"
	},
	"recommendPreloadInfo": {
	"preloadFlag": "3",
	"joinliveGetliverelatedlistPullType": 2
	},
	"finderJoinliveTraceBuffer": "CLO0lKns6NjnzwESGQiztJSp7OjY588BENy1mNMGGNy1mNMGIAA=",
	"promoteExtInfo": {
	"skipReport": false,
	"reportDelayInterval": 49
	},
	"liveExtFlagInfo": {
	"micExtFlag": "1124073",
	"showFlag": "0",
	"msgExtFlag": "212",
	"screenOrientationExtFlag": "1",
	"createLivePrepareSwitchFlag": "31"
	},
	"switchHideIdentityJumpInfo": {
	"jumpinfoType": 6,
	"businessType": 39,
	"liteAppInfo": {
	"appId": "wxalitef0df6a1a351a98ad58c9b67ae4a98012",
	"path": "pages/global-level",
	"query": "idInvisibleOnly=1&liveId=2078967496773105135"
	},
	"style": [],
	"supportDeviceList": [],
	"schemaInfos": []
	},
	"isFanClubSuperFans": false,
	"serverTime": "1785076444",
	"modeInfo": {
	"liveMode": 1,
	"liveSubMode": 1
	},
	"respExtend": {
	"ktvExtInfo": {
	"ktvSoundEngineFlag": 1,
	"ktvVolumeStrategyFlag": 2,
	"ktvBluetoothStrategyFlag": 1
	},
	"buttonDisplayPriority": [
	14,
	20,
	15,
	22
	],
	"isNeverJoinFanclub": true,
	"liveModeControlInfoList": [],
	"backendSeiInfo": {
	"seiInfos": [],
	"version": "1785076417091"
	},
	"emojiMsgConfigInfo": {
	"maxSize": 6,
	"minSize": 4,
	"isInlineComment": false,
	"barrageMaxSize": 3
	},
	"platformReminderNotificationInfo": [],
	"doubleClickLikeGuideForBeginners": {
	"timeIntervalOfGuidanceAfterJoinLiveS": 30
	}
	}
}`

func TestToAccount_FromLiveFeedJSON(t *testing.T) {
	var obj wxchannelspkg.ChannelsObject
	require.NoError(t, json.Unmarshal([]byte(liveFeedJSON), &obj))

	account, err := wxchannels.ToAccount(&obj)
	require.NoError(t, err)
	require.NotNil(t, account)

	// Live object with anchorContact: pickAccountContact prefers AnchorContact
	assert.Equal(t, "wx_channels:anchor_user_123", account.Id)
	assert.Equal(t, "anchor_user_123", account.Username)
	assert.Equal(t, "anchor_user_123", account.ExternalId)
	assert.Equal(t, "主播昵称", account.Nickname)
	assert.Equal(t, "https://wx.qlogo.cn/finderhead/anchor_avatar.jpg", account.AvatarURL)
	assert.Equal(t, "wx_channels", account.PlatformId)
}

func TestToContent_FromLiveFeedJSON(t *testing.T) {
	var obj wxchannelspkg.ChannelsObject
	require.NoError(t, json.Unmarshal([]byte(liveFeedJSON), &obj))

	content, err := wxchannels.ToContent(&obj)
	require.NoError(t, err)
	require.NotNil(t, content)

	assert.Equal(t, "wx_channels:14962698468287781449", content.Id)
	assert.Equal(t, "wx_channels", content.PlatformId)
	assert.Equal(t, "14962698468287781449", content.ExternalId)
	assert.Equal(t, "live_nonce_123_0", content.ExternalId2)
	assert.Equal(t, "live", content.ContentType)
	assert.Equal(t, "直播", content.Title)
	assert.Equal(t, "https://example.com/anchor_cover.jpg", content.CoverURL)

	require.NotNil(t, content.PublishTime)
	assert.Equal(t, int64(1785075244), *content.PublishTime)
}

// Test live feed without anchorContact: should fall back to Contact
func TestToAccount_FromLiveFeed_NoAnchorContact(t *testing.T) {
	payload := `{
		"id": "123",
		"nickname": "顶层昵称",
		"username": "top_user",
		"contact": {
			"username": "contact_user",
			"nickname": "联系人昵称",
			"headUrl": "http://example.com/contact.jpg"
		},
		"liveInfo": {
			"anchorStatusFlag": "123"
		}
	}`

	var obj wxchannelspkg.ChannelsObject
	require.NoError(t, json.Unmarshal([]byte(payload), &obj))

	account, err := wxchannels.ToAccount(&obj)
	require.NoError(t, err)
	require.NotNil(t, account)

	assert.Equal(t, "联系人昵称", account.Nickname)
	assert.Equal(t, "contact_user", account.Username)
	assert.Equal(t, "http://example.com/contact.jpg", account.AvatarURL)
}

// Test non-live object with anchorContact: should fall back to Contact (anchorContact only used for live)
func TestToAccount_FromNonLive_WithAnchorContact(t *testing.T) {
	payload := `{
		"id": "123",
		"nickname": "顶层昵称",
		"username": "top_user",
		"contact": {
			"username": "contact_user",
			"nickname": "联系人昵称"
		},
		"anchorContact": {
			"username": "anchor_user",
			"nickname": "主播"
		},
		"objectDesc": {
			"description": "视频",
			"mediaType": 4,
			"media": [
				{
					"url": "https://example.com/video.mp4",
					"thumbUrl": "https://example.com/thumb.jpg",
					"fileSize": 100,
					"videoPlayLen": 10,
					"width": 1920,
					"height": 1080,
					"decodeKey": "key123"
				}
			]
		}
	}`

	var obj wxchannelspkg.ChannelsObject
	require.NoError(t, json.Unmarshal([]byte(payload), &obj))

	account, err := wxchannels.ToAccount(&obj)
	require.NoError(t, err)
	require.NotNil(t, account)

	// No liveInfo, so anchorContact is NOT preferred — falls back to Contact
	assert.Equal(t, "联系人昵称", account.Nickname)
	assert.Equal(t, "contact_user", account.Username)
}
